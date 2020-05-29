// +build darwin windows

package util

import (
	"github.com/pkg/errors"
)

// FindDeviceNodes is not implemented anywhere except Linux.
func FindDeviceNodes() (map[string]string, error) {
	return nil, errors.Errorf("not supported on non-Linux OSes")
}

// CheckRootlessUIDRange is not implemented anywhere except Linux.
func CheckRootlessUIDRange(uid int) error {
	return nil
}
