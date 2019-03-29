// +build !linux

package libpod

import (
	"github.com/pkg/errors"
)

func systemdSliceFromPath(parent, name string) (string, error) {
	return "", errors.Wrapf(ErrOSNotSupported, "cgroups are not supported on non-linux OSes")
}

func makeSystemdCgroup(path string) error {
	return errors.Wrapf(ErrOSNotSupported, "cgroups are not supported on non-linux OSes")
}

func deleteSystemdCgroup(path string) error {
	return errors.Wrapf(ErrOSNotSupported, "cgroups are not supported on non-linux OSes")
}

func assembleSystemdCgroupName(baseSlice, newSlice string) (string, error) {
	return "", errors.Wrapf(ErrOSNotSupported, "cgroups are not supported on non-linux OSes")
}

// LabelVolumePath takes a mount path for a volume and gives it an
// selinux label of either shared or not
func LabelVolumePath(path string, shared bool) error {
	return ErrNotImplemented
}
