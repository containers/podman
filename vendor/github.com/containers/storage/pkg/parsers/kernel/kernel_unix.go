// Package kernel provides helper function to get, parse and compare kernel
// versions for different platforms.
package kernel

import (
	"github.com/sirupsen/logrus"
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

// CheckKernelVersion checks if current kernel is newer than (or equal to)
// the given version.
func CheckKernelVersion(k, major, minor int) bool {
	if v, err := GetKernelVersion(); err != nil {
		logrus.Warnf("Error getting kernel version: %s", err)
	} else {
		if CompareKernelVersion(*v, VersionInfo{Kernel: k, Major: major, Minor: minor}) < 0 {
			return false
		}
	}
	return true
}
