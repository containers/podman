//go:build darwin || windows
// +build darwin windows

package util

import "errors"

// FindDeviceNodes is not implemented anywhere except Linux.
func FindDeviceNodes() (map[string]string, error) {
	return nil, errors.New("not supported on non-Linux OSes")
}
