// +build linux darwin

package util

// TODO once rootless function is consolidated under libpod, we
//  should work to take darwin from this

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/pkg/errors"
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
		uid := fmt.Sprintf("%d", rootless.GetRootlessUID())
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
				rootlessRuntimeDirError = fmt.Errorf("neither XDG_RUNTIME_DIR nor HOME was set non-empty")
				return
			}
			resolvedHome, err := filepath.EvalSymlinks(home)
			if err != nil {
				rootlessRuntimeDirError = errors.Wrapf(err, "cannot resolve %s", home)
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
				rootlessConfigHomeDirError = errors.Wrapf(err, "cannot resolve %s", home)
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
// DEPRECATED - switch to GetRootlessPauseProcessPidPathGivenDir
func GetRootlessPauseProcessPidPath() (string, error) {
	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "libpod", "pause.pid"), nil
}

// GetRootlessPauseProcessPidPathGivenDir returns the path to the file that
// holds the PID of the pause process, given the location of Libpod's temporary
// files.
func GetRootlessPauseProcessPidPathGivenDir(libpodTmpDir string) (string, error) {
	if libpodTmpDir == "" {
		return "", errors.Errorf("must provide non-empty temporary directory")
	}
	return filepath.Join(libpodTmpDir, "pause.pid"), nil
}
