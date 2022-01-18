//go:build linux
// +build linux

package libpod

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/podman/v4/pkg/namespaces"
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
	user, _, _, err := c.generateUserPasswdEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, user, "123:*:123:456:container user:/:/bin/sh\n")

	c.config.User = "567"
	user, _, _, err = c.generateUserPasswdEntry(0)
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
	group, _, err := c.generateUserGroupEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, group, "456:x:456:123\n")

	c.config.User = "567"
	group, _, err = c.generateUserGroupEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, group, "567:x:567:567\n")
}

func TestAppendLocalhost(t *testing.T) {
	{
		c := Container{
			config: &ContainerConfig{
				ContainerNetworkConfig: ContainerNetworkConfig{
					NetMode: namespaces.NetworkMode("slirp4netns"),
				},
			},
		}

		assert.Equal(t, "127.0.0.1\tlocalhost\n::1\tlocalhost\n", c.appendLocalhost(""))
		assert.Equal(t, "127.0.0.1\tlocalhost", c.appendLocalhost("127.0.0.1\tlocalhost"))
	}
	{
		c := Container{
			config: &ContainerConfig{
				ContainerNetworkConfig: ContainerNetworkConfig{
					NetMode: namespaces.NetworkMode("host"),
				},
			},
		}

		assert.Equal(t, "", c.appendLocalhost(""))
		assert.Equal(t, "127.0.0.1\tlocalhost", c.appendLocalhost("127.0.0.1\tlocalhost"))
	}
}
