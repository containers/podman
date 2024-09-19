//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/containers/podman/v5/pkg/systemd/quadlet"
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
	u, err := user.Current()
	assert.Nil(t, err)
	uidInt, err := strconv.Atoi(u.Uid)
	assert.Nil(t, err)

	if os.Getenv("_UNSHARED") != "true" {
		unitDirs := getUnitDirs(false)

		resolvedUnitDirAdminUser := resolveUnitDirAdminUser()
		userLevelFilter := getUserLevelFilter(resolvedUnitDirAdminUser)
		rootDirs := make(map[string]struct{}, 0)
		appendSubPaths(rootDirs, quadlet.UnitDirTemp, false, userLevelFilter)
		appendSubPaths(rootDirs, quadlet.UnitDirAdmin, false, userLevelFilter)
		appendSubPaths(rootDirs, quadlet.UnitDirDistro, false, userLevelFilter)
		assert.Equal(t, rootDirs, unitDirs, "rootful unit dirs should match")

		configDir, err := os.UserConfigDir()
		assert.Nil(t, err)

		rootlessDirs := make(map[string]struct{}, 0)

		systemUserDirLevel := len(strings.Split(resolvedUnitDirAdminUser, string(os.PathSeparator)))
		nonNumericFilter := getNonNumericFilter(resolvedUnitDirAdminUser, systemUserDirLevel)

		runtimeDir, found := os.LookupEnv("XDG_RUNTIME_DIR")
		if found {
			appendSubPaths(rootlessDirs, path.Join(runtimeDir, "containers/systemd"), false, nil)
		}
		appendSubPaths(rootlessDirs, path.Join(configDir, "containers/systemd"), false, nil)
		appendSubPaths(rootlessDirs, filepath.Join(quadlet.UnitDirAdmin, "users"), true, nonNumericFilter)
		appendSubPaths(rootlessDirs, filepath.Join(quadlet.UnitDirAdmin, "users", u.Uid), true, userLevelFilter)
		rootlessDirs[filepath.Join(quadlet.UnitDirAdmin, "users")] = struct{}{}

		unitDirs = getUnitDirs(true)
		assert.Equal(t, rootlessDirs, unitDirs, "rootless unit dirs should match")

		// Test that relative path returns an empty list
		t.Setenv("QUADLET_UNIT_DIRS", "./relative/path")
		unitDirs = getUnitDirs(false)
		assert.Equal(t, map[string]struct{}{}, unitDirs)

		name, err := os.MkdirTemp("", "dir")
		assert.Nil(t, err)
		// remove the temporary directory at the end of the program
		defer os.RemoveAll(name)

		t.Setenv("QUADLET_UNIT_DIRS", name)
		unitDirs = getUnitDirs(false)
		assert.Equal(t, map[string]struct{}{name: {}}, unitDirs, "rootful should use environment variable")

		unitDirs = getUnitDirs(true)
		assert.Equal(t, map[string]struct{}{name: {}}, unitDirs, "rootless should use environment variable")

		symLinkTestBaseDir, err := os.MkdirTemp("", "podman-symlinktest")
		assert.Nil(t, err)
		// remove the temporary directory at the end of the program
		defer os.RemoveAll(symLinkTestBaseDir)

		actualDir := filepath.Join(symLinkTestBaseDir, "actual")
		err = os.Mkdir(actualDir, 0755)
		assert.Nil(t, err)
		innerDir := filepath.Join(actualDir, "inner")
		err = os.Mkdir(innerDir, 0755)
		assert.Nil(t, err)
		symlink := filepath.Join(symLinkTestBaseDir, "symlink")
		err = os.Symlink(actualDir, symlink)
		assert.Nil(t, err)
		t.Setenv("QUADLET_UNIT_DIRS", symlink)
		unitDirs = getUnitDirs(true)
		assert.Equal(t, map[string]struct{}{actualDir: {}, innerDir: {}}, unitDirs, "directory resolution should follow symlink")

		// Make a more elborate test with the following structure:
		// <BASE>/linkToDir - real directory to link to
		// <BASE>/linkToDir/a - real directory
		// <BASE>/linkToDir/b - link to <BASE>/unitDir/b/a should be ignored
		// <BASE>/linkToDir/c - link to <BASE>/unitDir should be ignored
		// <BASE>/unitDir - start from here
		// <BASE>/unitDir/a - real directory
		// <BASE>/unitDir/a/a - real directory
		// <BASE>/unitDir/a/a/a - real directory
		// <BASE>/unitDir/b/a - real directory
		// <BASE>/unitDir/b/b - link to <BASE>/unitDir/a/a should be ignored
		// <BASE>/unitDir/c - link to <BASE>/linkToDir
		symLinkRecursiveTestBaseDir, err := os.MkdirTemp("", "podman-symlink-recursive-test")
		assert.Nil(t, err)
		// remove the temporary directory at the end of the program
		defer os.RemoveAll(symLinkRecursiveTestBaseDir)

		createDir := func(path, name string, dirs map[string]struct{}) string {
			dirName := filepath.Join(path, name)
			assert.NotContains(t, dirs, dirName)
			err = os.Mkdir(dirName, 0755)
			assert.Nil(t, err)
			dirs[dirName] = struct{}{}
			return dirName
		}
		expectedDirs := make(map[string]struct{}, 0)
		// Create <BASE>/linkToDir
		linkToDirPath := createDir(symLinkRecursiveTestBaseDir, "linkToDir", expectedDirs)
		// Create <BASE>/linkToDir/a
		createDir(linkToDirPath, "a", expectedDirs)
		// Create <BASE>/unitDir
		unitsDirPath := createDir(symLinkRecursiveTestBaseDir, "unitsDir", expectedDirs)
		// Create <BASE>/unitDir/a
		aDirPath := createDir(unitsDirPath, "a", expectedDirs)
		// Create <BASE>/unitDir/a/a
		aaDirPath := createDir(aDirPath, "a", expectedDirs)
		// Create <BASE>/unitDir/a/b
		createDir(aDirPath, "b", expectedDirs)
		// Create <BASE>/unitDir/a/a/a
		createDir(aaDirPath, "a", expectedDirs)
		// Create <BASE>/unitDir/b
		bDirPath := createDir(unitsDirPath, "b", expectedDirs)
		// Create <BASE>/unitDir/b/a
		baDirPath := createDir(bDirPath, "a", expectedDirs)

		linkDir := func(path, name, target string) {
			linkName := filepath.Join(path, name)
			err = os.Symlink(target, linkName)
			assert.Nil(t, err)
		}
		// Link <BASE>/unitDir/b/b to <BASE>/unitDir/a/a
		linkDir(bDirPath, "b", aaDirPath)
		// Link <BASE>/linkToDir/b to <BASE>/unitDir/b/a
		linkDir(linkToDirPath, "b", baDirPath)
		// Link <BASE>/linkToDir/c to <BASE>/unitDir
		linkDir(linkToDirPath, "c", unitsDirPath)
		// Link <BASE>/unitDir/c to <BASE>/linkToDir
		linkDir(unitsDirPath, "c", linkToDirPath)

		t.Setenv("QUADLET_UNIT_DIRS", unitsDirPath)
		unitDirs = getUnitDirs(true)
		assert.Equal(t, expectedDirs, unitDirs, "directory resolution should follow symlink")
		// remove the temporary directory at the end of the program
		defer os.RemoveAll(symLinkTestBaseDir)

		// because chroot is only available for root,
		// unshare the namespace and map user to root
		c := exec.Command("/proc/self/exe", os.Args[1:]...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER,
			UidMappings: []syscall.SysProcIDMap{
				{
					ContainerID: 0,
					HostID:      uidInt,
					Size:        1,
				},
			},
		}
		c.Env = append(os.Environ(), "_UNSHARED=true")
		err = c.Run()
		assert.Nil(t, err)
	} else {
		fmt.Println(os.Args)

		symLinkTestBaseDir, err := os.MkdirTemp("", "podman-symlinktest2")
		assert.Nil(t, err)
		defer os.RemoveAll(symLinkTestBaseDir)
		rootF, err := os.Open("/")
		assert.Nil(t, err)
		defer rootF.Close()
		defer func() {
			err := rootF.Chdir()
			assert.Nil(t, err)
			err = syscall.Chroot(".")
			assert.Nil(t, err)
		}()
		err = syscall.Chroot(symLinkTestBaseDir)
		assert.Nil(t, err)

		err = os.MkdirAll(quadlet.UnitDirAdmin, 0755)
		assert.Nil(t, err)
		err = os.RemoveAll(quadlet.UnitDirAdmin)
		assert.Nil(t, err)

		systemdDir := filepath.Join("/", "systemd")
		userDir := filepath.Join("/", "users")
		err = os.Mkdir(systemdDir, 0755)
		assert.Nil(t, err)
		err = os.Mkdir(userDir, 0755)
		assert.Nil(t, err)
		err = os.Symlink(userDir, filepath.Join(systemdDir, "users"))
		assert.Nil(t, err)
		err = os.Symlink(systemdDir, quadlet.UnitDirAdmin)
		assert.Nil(t, err)

		uidDir := filepath.Join(userDir, u.Uid)
		err = os.Mkdir(uidDir, 0755)
		assert.Nil(t, err)
		uidDir2 := filepath.Join(userDir, strconv.Itoa(uidInt+1))
		err = os.Mkdir(uidDir2, 0755)
		assert.Nil(t, err)

		t.Setenv("QUADLET_UNIT_DIRS", "")
		unitDirs := getUnitDirs(false)
		assert.NotContains(t, unitDirs, userDir, "rootful should not contain rootless")
		unitDirs = getUnitDirs(true)
		assert.NotContains(t, unitDirs, uidDir2, "rootless should not contain other users'")
	}
}
