// +build linux darwin

package util

// TODO once rootless function is consolidated under libpod, we
//  should work to take darwin from this

import (
	"fmt"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"syscall"
)

// GetRootlessRuntimeDir returns the runtime directory when running as non root
func GetRootlessRuntimeDir() (string, error) {
	var rootlessRuntimeDirError error

	rootlessRuntimeDirOnce.Do(func() {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		uid := fmt.Sprintf("%d", rootless.GetRootlessUID())
		if runtimeDir == "" {
			tmpDir := filepath.Join("/run", "user", uid)
			os.MkdirAll(tmpDir, 0700)
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && st.Mode().Perm() == 0700 {
				runtimeDir = tmpDir
			}
		}
		if runtimeDir == "" {
			tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("run-%s", uid))
			os.MkdirAll(tmpDir, 0700)
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && st.Mode().Perm() == 0700 {
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
