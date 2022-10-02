//go:build freebsd
// +build freebsd

package libpod

import (
	"errors"
	"syscall"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// systemdSliceFromPath makes a new systemd slice under the given parent with
// the given name.
// The parent must be a slice. The name must NOT include ".slice"
func systemdSliceFromPath(parent, name string, resources *spec.LinuxResources) (string, error) {
	return "", errors.New("not implemented systemdSliceFromPath")
}

// deleteSystemdCgroup deletes the systemd cgroup at the given location
func deleteSystemdCgroup(path string, resources *spec.LinuxResources) error {
	return nil
}

// No equivalent on FreeBSD?
func LabelVolumePath(path string) error {
	return nil
}

// Unmount umounts a target directory
func Unmount(mount string) {
	if err := unix.Unmount(mount, unix.MNT_FORCE); err != nil {
		if err != syscall.EINVAL {
			logrus.Warnf("Failed to unmount %s : %v", mount, err)
		} else {
			logrus.Debugf("failed to unmount %s : %v", mount, err)
		}
	}
}
