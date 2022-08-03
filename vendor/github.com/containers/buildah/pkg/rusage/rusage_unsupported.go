// +build windows

package rusage

import (
	"syscall"

	"github.com/pkg/errors"
)

func get() (Rusage, error) {
	return Rusage{}, errors.Wrapf(syscall.ENOTSUP, "error getting resource usage")
}

// Supported returns true if resource usage counters are supported on this OS.
func Supported() bool {
	return false
}
