//go:build linux
// +build linux

package libpod

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/namespaces"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/stringid"
	"github.com/google/go-cmp/cmp"
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

func TestContainerUUIDEnv(t *testing.T) {
	ctx := context.Background()
	c := makeTestContainer(t)

	s, err := c.generateSpec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, hasProcessEnv(t, s, "container_uuid="+c.config.ID[:32]))

	c.config.Spec.Process.Env = append(c.config.Spec.Process.Env, "container_uuid=test")
	s, err = c.generateSpec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, hasProcessEnv(t, s, "container_uuid=test"))
}

func TestContainerRunHost(t *testing.T) {
	ctx := context.Background()
	c := makeTestContainer(t)

	s, err := c.generateSpec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, hasMount(t, s, spec.Mount{
		Destination: "/run/host",
		Source:      c.runHostDir(),
		Type:        "bind",
		Options:     []string{"bind", "rprivate", "ro", "nosuid", "noexec", "nodev"}},
	), "run-host mount not found: %+v", s.Mounts)
	b, err := os.ReadFile(filepath.Join(c.runHostDir(), "container-manager"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	assert.Equal(t, string(b), "podman\n")
	b, err = os.ReadFile(filepath.Join(c.runHostDir(), "container-uuid"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	assert.Equal(t, string(b), c.config.ID[:32]+"\n")
}

func makeTestContainer(t *testing.T) Container {
	t.Helper()
	return Container{
		config: &ContainerConfig{
			Spec: &spec.Spec{
				Process: &spec.Process{},
				Root:    &spec.Root{},
				Linux: &spec.Linux{
					Namespaces: []spec.LinuxNamespace{{Type: spec.NetworkNamespace, Path: ""}},
				},
			},
			ID: stringid.GenerateNonCryptoID(),
			IDMappings: storage.IDMappingOptions{
				UIDMap: []idtools.IDMap{{ContainerID: 0, HostID: os.Geteuid(), Size: 1}},
				GIDMap: []idtools.IDMap{{ContainerID: 0, HostID: os.Getegid(), Size: 1}},
			},
			ContainerNetworkConfig: ContainerNetworkConfig{UseImageHosts: true},
			ContainerMiscConfig:    ContainerMiscConfig{CgroupManager: config.SystemdCgroupsManager},
		},
		state: &ContainerState{
			BindMounts: map[string]string{"/run/.containerenv": ""},
			RunDir:     t.TempDir(),
		},
		runtime: &Runtime{
			config: &config.Config{Containers: config.ContainersConfig{}},
		},
	}
}

func hasProcessEnv(t *testing.T, s *spec.Spec, want string) bool {
	t.Helper()
	for _, checkEnv := range s.Process.Env {
		if checkEnv == want {
			return true
		}
	}
	return false
}

func hasMount(t *testing.T, s *spec.Spec, want spec.Mount) bool {
	t.Helper()
	for _, m := range s.Mounts {
		if cmp.Equal(want, m) {
			return true
		}
	}
	return false
}
