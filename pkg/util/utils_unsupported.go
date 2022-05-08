//go:build darwin || windows || freebsd
// +build darwin windows freebsd

package util

import "errors"

// FindDeviceNodes is not implemented anywhere except Linux.
func FindDeviceNodes() (map[string]string, error) {
	return nil, errors.New("not supported on non-Linux OSes")
}
