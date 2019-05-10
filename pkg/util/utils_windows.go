// +build windows

package util

import (
	"github.com/pkg/errors"
)

// GetRootlessRuntimeDir returns the runtime directory when running as non root
func GetRootlessRuntimeDir() (string, error) {
	return "", errors.New("this function is not implemented for windows")
}

// IsCgroup2UnifiedMode returns whether we are running in cgroup 2 unified mode.
func IsCgroup2UnifiedMode() (bool, error) {
	return false, errors.New("this function is not implemented for windows")
}
