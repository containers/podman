//go:build !remote

package libpod

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
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
