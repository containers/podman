//go:build !linux && !(freebsd && cgo)
// +build !linux
// +build !freebsd !cgo

package chroot

import (
	"errors"
)

func getPtyDescriptors() (int, int, error) {
	return -1, -1, errors.New("getPtyDescriptors not supported on this platform")
}
