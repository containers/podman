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
