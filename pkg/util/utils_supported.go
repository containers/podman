// +build !windows

package util

// TODO once rootless function is consolidated under libpod, we
//  should work to take darwin from this

import (
	"os"
	"path/filepath"
	"syscall"

	cutil "github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/pkg/errors"
)

// GetRuntimeDir returns the runtime directory
func GetRuntimeDir() (string, error) {
	if !rootless.IsRootless() {
		return "", nil
	}
	return cutil.GetRuntimeDir()
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
