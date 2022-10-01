//go:build freebsd || openbsd
// +build freebsd openbsd

package kernel

import (
	"errors"
)

// A stub called by kernel_unix.go .
func uname() (*Utsname, error) {
	return nil, errors.New("Kernel version detection is available only on linux")
}
