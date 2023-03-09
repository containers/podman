//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"errors"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// systemdSliceFromPath makes a new systemd slice under the given parent with
// the given name.
// The parent must be a slice. The name must NOT include ".slice"
func systemdSliceFromPath(parent, name string, resources *spec.LinuxResources) (string, error) {
	return "", errors.New("not implemented systemdSliceFromPath")
}

// Unmount umounts a target directory
func Unmount(mount string) {
}

// LabelVolumePath takes a mount path for a volume and gives it an
// selinux label of either shared or not
func LabelVolumePath(path, mountLabel string) error {
	return errors.New("not implemented LabelVolumePath")
}
