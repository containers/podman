package main

import (
	"os"
	"os/user"
	"path"
	"path/filepath"
	"testing"

	"github.com/containers/podman/v4/pkg/systemd/quadlet"
	"github.com/stretchr/testify/assert"
)

func TestIsUnambiguousName(t *testing.T) {
	tests := []struct {
		input string
		res   bool
	}{
		// Ambiguous names
		{"fedora", false},
		{"fedora:latest", false},
		{"library/fedora", false},
		{"library/fedora:latest", false},
		{"busybox@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", false},
		{"busybox:latest@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", false},
		{"d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05", false},
		{"d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05aa", false},

		// Unambiguous names
		{"quay.io/fedora", true},
		{"docker.io/fedora", true},
		{"docker.io/library/fedora:latest", true},
		{"localhost/fedora", true},
		{"localhost:5000/fedora:latest", true},
		{"example.foo.this.may.be.garbage.but.maybe.not:1234/fedora:latest", true},
		{"docker.io/library/busybox@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
		{"docker.io/library/busybox:latest@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
		{"docker.io/fedora@sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
		{"sha256:d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
		{"d366a4665ab44f0648d7a00ae3fae139d55e32f9712c67accd604bb55df9d05a", true},
	}

	for _, test := range tests {
		res := isUnambiguousName(test.input)
		assert.Equal(t, res, test.res, "%q", test.input)
	}
}

func TestUnitDirs(t *testing.T) {
	rootDirs := []string{}
	rootDirs = appendSubPaths(rootDirs, quadlet.UnitDirAdmin, false, userLevelFilter)
	rootDirs = appendSubPaths(rootDirs, quadlet.UnitDirDistro, false, userLevelFilter)
	unitDirs := getUnitDirs(false)
	assert.Equal(t, unitDirs, rootDirs, "rootful unit dirs should match")

	configDir, err := os.UserConfigDir()
	assert.Nil(t, err)
	u, err := user.Current()
	assert.Nil(t, err)

	rootlessDirs := []string{}

	rootlessDirs = appendSubPaths(rootlessDirs, path.Join(configDir, "containers/systemd"), false, nil)
	rootlessDirs = appendSubPaths(rootlessDirs, filepath.Join(quadlet.UnitDirAdmin, "users"), true, nonNumericFilter)
	rootlessDirs = appendSubPaths(rootlessDirs, filepath.Join(quadlet.UnitDirAdmin, "users", u.Uid), true, userLevelFilter)
	rootlessDirs = append(rootlessDirs, filepath.Join(quadlet.UnitDirAdmin, "users"))

	unitDirs = getUnitDirs(true)
	assert.Equal(t, unitDirs, rootlessDirs, "rootless unit dirs should match")

	name, err := os.MkdirTemp("", "dir")
	assert.Nil(t, err)
	// remove the temporary directory at the end of the program
	defer os.RemoveAll(name)

	t.Setenv("QUADLET_UNIT_DIRS", name)
	unitDirs = getUnitDirs(false)
	assert.Equal(t, unitDirs, []string{name}, "rootful should use environment variable")

	unitDirs = getUnitDirs(true)
	assert.Equal(t, unitDirs, []string{name}, "rootless should use environment variable")
}
