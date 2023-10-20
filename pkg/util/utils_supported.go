//go:build !windows
// +build !windows

package util

// TODO once rootless function is consolidated under libpod, we
//  should work to take darwin from this

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/sirupsen/logrus"
)

// GetRuntimeDir returns the runtime directory
func GetRuntimeDir() (string, error) {
	var rootlessRuntimeDirError error

	if !rootless.IsRootless() {
		return "", nil
	}

	rootlessRuntimeDirOnce.Do(func() {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")

		if runtimeDir != "" {
			rootlessRuntimeDir, rootlessRuntimeDirError = filepath.EvalSymlinks(runtimeDir)
			return
		}

		uid := strconv.Itoa(rootless.GetRootlessUID())
		if runtimeDir == "" {
			tmpDir := filepath.Join("/run", "user", uid)
			if err := os.MkdirAll(tmpDir, 0700); err != nil {
				logrus.Debug(err)
			}
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && (st.Mode().Perm()&0700 == 0700) {
				runtimeDir = tmpDir
			}
		}
		if runtimeDir == "" {
			tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("podman-run-%s", uid))
			if err := os.MkdirAll(tmpDir, 0700); err != nil {
				logrus.Debug(err)
			}
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && (st.Mode().Perm()&0700 == 0700) {
				runtimeDir = tmpDir
			}
		}
		if runtimeDir == "" {
			home := os.Getenv("HOME")
			if home == "" {
				rootlessRuntimeDirError = errors.New("neither XDG_RUNTIME_DIR nor HOME was set non-empty")
				return
			}
			resolvedHome, err := filepath.EvalSymlinks(home)
			if err != nil {
				rootlessRuntimeDirError = fmt.Errorf("cannot resolve %s: %w", home, err)
				return
			}
			runtimeDir = filepath.Join(resolvedHome, "rundir")
		}
		rootlessRuntimeDir = runtimeDir
	})

	if rootlessRuntimeDirError != nil {
		return "", rootlessRuntimeDirError
	}
	return rootlessRuntimeDir, nil
}

// GetRootlessConfigHomeDir returns the config home directory when running as non root
func GetRootlessConfigHomeDir() (string, error) {
	var rootlessConfigHomeDirError error

	rootlessConfigHomeDirOnce.Do(func() {
		cfgHomeDir := os.Getenv("XDG_CONFIG_HOME")
		if cfgHomeDir == "" {
			home := os.Getenv("HOME")
			resolvedHome, err := filepath.EvalSymlinks(home)
			if err != nil {
				rootlessConfigHomeDirError = fmt.Errorf("cannot resolve %s: %w", home, err)
				return
			}
			tmpDir := filepath.Join(resolvedHome, ".config")
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && st.Mode().Perm() >= 0700 {
				cfgHomeDir = tmpDir
			}
		}
		rootlessConfigHomeDir = cfgHomeDir
	})

	if rootlessConfigHomeDirError != nil {
		return "", rootlessConfigHomeDirError
	}

	return rootlessConfigHomeDir, nil
}

// GetRootlessPauseProcessPidPath returns the path to the file that holds the pid for
// the pause process.
func GetRootlessPauseProcessPidPath() (string, error) {
	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		return "", err
	}
	// Note this path must be kept in sync with pkg/rootless/rootless_linux.go
	// We only want a single pause process per user, so we do not want to use
	// the tmpdir which can be changed via --tmpdir.
	return filepath.Join(runtimeDir, "libpod", "tmp", "pause.pid"), nil
}
