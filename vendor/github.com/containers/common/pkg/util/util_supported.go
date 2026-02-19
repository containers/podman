//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package util

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/unshare"
	"github.com/sirupsen/logrus"
)

var (
	rootlessRuntimeDirOnce sync.Once
	rootlessRuntimeDir     string
)

// isWriteableOnlyByOwner checks that the specified permission mask allows write
// access only to the owner.
func isWriteableOnlyByOwner(perm os.FileMode) bool {
	return (perm & 0o722) == 0o700
}

// GetRuntimeDir returns the runtime directory
func GetRuntimeDir() (string, error) {
	var rootlessRuntimeDirError error

	rootlessRuntimeDirOnce.Do(func() {
		runtimeDir, err := homedir.GetRuntimeDir()
		if err != nil {
			logrus.Debug(err)
		}
		if runtimeDir != "" {
			st, err := os.Stat(runtimeDir)
			if err != nil {
				rootlessRuntimeDirError = err
				return
			}
			if int(st.Sys().(*syscall.Stat_t).Uid) != os.Geteuid() {
				rootlessRuntimeDirError = fmt.Errorf("XDG_RUNTIME_DIR directory %q is not owned by the current user", runtimeDir)
				return
			}
		}
		uid := fmt.Sprintf("%d", unshare.GetRootlessUID())
		if runtimeDir == "" {
			tmpDir := filepath.Join("/run", "user", uid)
			if err := os.MkdirAll(tmpDir, 0o700); err != nil {
				logrus.Debugf("unable to make temp dir: %v", err)
			}
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && isWriteableOnlyByOwner(st.Mode().Perm()) {
				runtimeDir = tmpDir
			}
		}
		if runtimeDir == "" {
			tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("podman-run-%s", uid))
			if err := os.MkdirAll(tmpDir, 0o700); err != nil {
				logrus.Debugf("unable to make temp dir %v", err)
			}
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && isWriteableOnlyByOwner(st.Mode().Perm()) {
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
				rootlessRuntimeDirError = fmt.Errorf("cannot resolve home: %w", err)
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
