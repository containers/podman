//go:build !remote

package libpod

import (
	"fmt"
	"os"
	"testing"
	"unicode/utf8"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/types"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateUserPasswdEntry(t *testing.T) {
	c := Container{
		config: &ContainerConfig{
			Spec: &spec.Spec{},
			ContainerSecurityConfig: ContainerSecurityConfig{
				User: "123456:456789",
			},
		},
		state: &ContainerState{
			Mountpoint: "/does/not/exist/tmp/",
		},
	}
	user, err := c.generateUserPasswdEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, user, "123456:*:123456:456789:container user:/:/bin/sh\n")

	c.config.User = "567890"
	user, err = c.generateUserPasswdEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, user, "567890:*:567890:0:container user:/:/bin/sh\n")
}

func TestGenerateUserGroupEntry(t *testing.T) {
	c := Container{
		config: &ContainerConfig{
			Spec: &spec.Spec{},
			ContainerSecurityConfig: ContainerSecurityConfig{
				User: "123456:456789",
			},
		},
		state: &ContainerState{
			Mountpoint: "/does/not/exist/tmp/",
		},
	}
	group, err := c.generateUserGroupEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, group, "456789:x:456789:123456\n")

	c.config.User = "567890"
	group, err = c.generateUserGroupEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, group, "567890:x:567890:567890\n")
}

func TestMakePlatformBindMounts(t *testing.T) {
	runDir, err := os.MkdirTemp(os.TempDir(), "rundir")
	require.NoError(t, err, "Unable to create temp directory")
	defer os.RemoveAll(runDir)
	testHostname := "test-hostname"
	c := Container{
		config: &ContainerConfig{
			Spec: &spec.Spec{
				Hostname: testHostname,
			},
			IDMappings: types.IDMappingOptions{
				UIDMap: []idtools.IDMap{
					{
						ContainerID: 0,
						HostID:      os.Getuid(),
					},
				},
				GIDMap: []idtools.IDMap{
					{
						ContainerID: 0,
						HostID:      os.Getgid(),
					},
				},
			},
		},
		state: &ContainerState{
			RunDir:     runDir,
			BindMounts: make(map[string]string),
		},
	}
	err = c.makePlatformBindMounts()
	require.NoError(t, err)

	bindMountEtcHostname, ok := c.state.BindMounts["/etc/hostname"]
	require.True(t, ok, "Unable to locate container /etc/hostname")
	require.Greater(t, len(bindMountEtcHostname), 0, "hostname file not configured")

	contents, err := os.ReadFile(bindMountEtcHostname)
	require.NoError(t, err, "error reading container /etc/hostname")
	require.Greater(t, len(contents), 0, "container /etc/hostname was empty")

	strContents := string(contents)
	require.True(t, utf8.ValidString(strContents), "hostname does not contain valid utf-8 string")
	require.Equal(t, fmt.Sprintf("%s\n", testHostname), strContents)
}
