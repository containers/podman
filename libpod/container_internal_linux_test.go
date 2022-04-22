//go:build linux
// +build linux

package libpod

import (
	"io/ioutil"
	"os"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestGenerateUserPasswdEntry(t *testing.T) {
	dir, err := ioutil.TempDir("", "libpod_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	c := Container{
		config: &ContainerConfig{
			Spec: &spec.Spec{},
			ContainerSecurityConfig: ContainerSecurityConfig{
				User: "123:456",
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
	assert.Equal(t, user, "123:*:123:456:container user:/:/bin/sh\n")

	c.config.User = "567"
	user, err = c.generateUserPasswdEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, user, "567:*:567:0:container user:/:/bin/sh\n")
}

func TestGenerateUserGroupEntry(t *testing.T) {
	c := Container{
		config: &ContainerConfig{
			Spec: &spec.Spec{},
			ContainerSecurityConfig: ContainerSecurityConfig{
				User: "123:456",
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
	assert.Equal(t, group, "456:x:456:123\n")

	c.config.User = "567"
	group, err = c.generateUserGroupEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, group, "567:x:567:567\n")
}
