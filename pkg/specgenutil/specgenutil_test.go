//go:build linux
// +build linux

package specgenutil

import (
	"testing"

	"github.com/containers/common/pkg/machine"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/stretchr/testify/assert"
)

func TestWinPath(t *testing.T) {
	const (
		fail = false
		pass = true
	)
	tests := []struct {
		vol     string
		source  string
		dest    string
		isN     bool
		outcome bool
		mach    string
	}{
		{`C:\Foo:/blah`, "/mnt/c/Foo", "/blah", false, pass, "wsl"},
		{`C:\Foo:/blah`, "/mnt/c/Foo", "/blah", false, fail, ""},
		{`\\?\C:\Foo:/blah`, "/mnt/c/Foo", "/blah", false, pass, "wsl"},
		{`/c/bar:/blah`, "/mnt/c/bar", "/blah", false, pass, "wsl"},
		{`/c/bar:/blah`, "/c/bar", "/blah", false, pass, ""},
		{`/test/this:/blah`, "/test/this", "/blah", false, pass, "wsl"},
		{`c:/bar/something:/other`, "/mnt/c/bar/something", "/other", false, pass, "wsl"},
		{`c:/foo:ro`, "c", "/foo", true, pass, ""},
		{`\\computer\loc:/dest`, "", "", false, fail, "wsl"},
		{`\\.\drive\loc:/target`, "/mnt/wsl/drive/loc", "/target", false, pass, "wsl"},
	}

	f := func(vol string, mach string) (*specgen.SpecGenerator, error) {
		machine := machine.GetMachineMarker()
		oldEnable, oldType := machine.Enabled, machine.Type
		machine.Enabled, machine.Type = len(mach) > 0, mach
		sg := specgen.NewSpecGenerator("nothing", false)
		err := FillOutSpecGen(sg, &entities.ContainerCreateOptions{
			ImageVolume: "ignore",
			Volume:      []string{vol}}, []string{},
		)
		machine.Enabled, machine.Type = oldEnable, oldType
		return sg, err
	}

	for _, test := range tests {
		msg := "Checking: " + test.vol
		sg, err := f(test.vol, test.mach)
		if test.outcome == fail {
			assert.NotNil(t, err, msg)
			continue
		}
		if !assert.Nil(t, err, msg) {
			continue
		}
		if test.isN {
			if !assert.Equal(t, 1, len(sg.Volumes), msg) {
				continue
			}
			assert.Equal(t, test.source, sg.Volumes[0].Name, msg)
			assert.Equal(t, test.dest, sg.Volumes[0].Dest, msg)
		} else {
			if !assert.Equal(t, 1, len(sg.Mounts), msg) {
				continue
			}
			assert.Equal(t, test.source, sg.Mounts[0].Source, msg)
			assert.Equal(t, test.dest, sg.Mounts[0].Destination, msg)
		}
	}
}

func TestParseLinuxResourcesDeviceAccess(t *testing.T) {
	d, err := parseLinuxResourcesDeviceAccess("a *:* rwm")
	assert.Nil(t, err, "err is nil")
	assert.True(t, d.Allow, "allow is true")
	assert.Equal(t, d.Type, "a", "type is 'a'")
	assert.Nil(t, d.Minor, "minor is nil")
	assert.Nil(t, d.Major, "major is nil")

	d, err = parseLinuxResourcesDeviceAccess("b 3:* rwm")
	assert.Nil(t, err, "err is nil")
	assert.True(t, d.Allow, "allow is true")
	assert.Equal(t, d.Type, "b", "type is 'b'")
	assert.Nil(t, d.Minor, "minor is nil")
	assert.NotNil(t, d.Major, "major is not nil")
	assert.Equal(t, *d.Major, int64(3), "major is 3")

	d, err = parseLinuxResourcesDeviceAccess("a *:3 rwm")
	assert.Nil(t, err, "err is nil")
	assert.True(t, d.Allow, "allow is true")
	assert.Equal(t, d.Type, "a", "type is 'a'")
	assert.Nil(t, d.Major, "major is nil")
	assert.NotNil(t, d.Minor, "minor is not nil")
	assert.Equal(t, *d.Minor, int64(3), "minor is 3")

	d, err = parseLinuxResourcesDeviceAccess("c 1:2 rwm")
	assert.Nil(t, err, "err is nil")
	assert.True(t, d.Allow, "allow is true")
	assert.Equal(t, d.Type, "c", "type is 'c'")
	assert.NotNil(t, d.Major, "minor is not nil")
	assert.Equal(t, *d.Major, int64(1), "minor is 1")
	assert.NotNil(t, d.Minor, "minor is not nil")
	assert.Equal(t, *d.Minor, int64(2), "minor is 2")

	_, err = parseLinuxResourcesDeviceAccess("q *:* rwm")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a a:* rwm")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:a rwm")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:* abc")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("* *:* *")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("* *:a2 *")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("*")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("*:*")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:*")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:*")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a 12a:* r")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a a12:* r")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a 0x1:* r")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a -2:* r")
	assert.NotNil(t, err, "err is not nil")

	_, err = parseLinuxResourcesDeviceAccess("a *:-3 r")
	assert.NotNil(t, err, "err is not nil")
}
