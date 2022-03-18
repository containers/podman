//go:build !linux
// +build !linux

package cgroups

import (
	"os"
)

// IsCgroup2UnifiedMode returns whether we are running in cgroup 2 cgroup2 mode.
func IsCgroup2UnifiedMode() (bool, error) {
	return false, nil
}

// UserOwnsCurrentSystemdCgroup checks whether the current EUID owns the
// current cgroup.
func UserOwnsCurrentSystemdCgroup() (bool, error) {
	return false, nil
}

func rmDirRecursively(path string) error {
	return os.RemoveAll(path)
}
