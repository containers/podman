// +build !linux

package libpod

import (
	"github.com/containers/podman/v2/libpod/define"
	"github.com/pkg/errors"
)

func systemdSliceFromPath(parent, name string) (string, error) {
	return "", errors.Wrapf(define.ErrOSNotSupported, "cgroups are not supported on non-linux OSes")
}

func makeSystemdCgroup(path string) error {
	return errors.Wrapf(define.ErrOSNotSupported, "cgroups are not supported on non-linux OSes")
}

func deleteSystemdCgroup(path string) error {
	return errors.Wrapf(define.ErrOSNotSupported, "cgroups are not supported on non-linux OSes")
}

func assembleSystemdCgroupName(baseSlice, newSlice string) (string, error) {
	return "", errors.Wrapf(define.ErrOSNotSupported, "cgroups are not supported on non-linux OSes")
}

// LabelVolumePath takes a mount path for a volume and gives it an
// selinux label of either shared or not
func LabelVolumePath(path string) error {
	return define.ErrNotImplemented
}

func Unmount(mount string) error {
	return define.ErrNotImplemented
}
