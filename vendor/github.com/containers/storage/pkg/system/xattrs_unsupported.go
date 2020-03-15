// +build !linux

package system

import "syscall"

const (
	// Value is larger than the maximum size allowed
	E2BIG syscall.Errno = syscall.Errno(0)

	// Operation not supported
	EOPNOTSUPP syscall.Errno = syscall.Errno(0)
)

// Lgetxattr is not supported on platforms other than linux.
func Lgetxattr(path string, attr string) ([]byte, error) {
	return nil, ErrNotSupportedPlatform
}

// Lsetxattr is not supported on platforms other than linux.
func Lsetxattr(path string, attr string, data []byte, flags int) error {
	return ErrNotSupportedPlatform
}

// Llistxattr is not supported on platforms other than linux.
func Llistxattr(path string) ([]string, error) {
	return nil, ErrNotSupportedPlatform
}
