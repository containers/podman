//go:build unix && !darwin

// Package kernel provides helper function to get, parse and compare kernel
// versions for different platforms.
package kernel

import (
	"golang.org/x/sys/unix"
)

// GetKernelVersion gets the current kernel version.
func GetKernelVersion() (*VersionInfo, error) {
	uts := &unix.Utsname{}

	if err := unix.Uname(uts); err != nil {
		return nil, err
	}

	return ParseRelease(unix.ByteSliceToString(uts.Release[:]))
}
