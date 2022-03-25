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
