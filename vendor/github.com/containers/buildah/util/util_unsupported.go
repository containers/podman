// +build !linux

package util

import (
	"os"
)

// IsCgroup2UnifiedMode returns whether we are running in cgroup 2 cgroup2 mode.
func IsCgroup2UnifiedMode() (bool, error) {
	return false, nil
}

func UID(st os.FileInfo) int {
	return 0
}

func GID(st os.FileInfo) int {
	return 0
}
