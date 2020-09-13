// +build linux

package libpod

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/cgroups"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// systemdSliceFromPath makes a new systemd slice under the given parent with
// the given name.
// The parent must be a slice. The name must NOT include ".slice"
func systemdSliceFromPath(parent, name string) (string, error) {
	cgroupPath, err := assembleSystemdCgroupName(parent, name)
	if err != nil {
		return "", err
	}

	logrus.Debugf("Created cgroup path %s for parent %s and name %s", cgroupPath, parent, name)

	if err := makeSystemdCgroup(cgroupPath); err != nil {
		return "", errors.Wrapf(err, "error creating cgroup %s", cgroupPath)
	}

	logrus.Debugf("Created cgroup %s", cgroupPath)

	return cgroupPath, nil
}

func getDefaultSystemdCgroup() string {
	if rootless.IsRootless() {
		return SystemdDefaultRootlessCgroupParent
	}
	return SystemdDefaultCgroupParent
}

// makeSystemdCgroup creates a systemd CGroup at the given location.
func makeSystemdCgroup(path string) error {
	controller, err := cgroups.NewSystemd(getDefaultSystemdCgroup())
	if err != nil {
		return err
	}

	if rootless.IsRootless() {
		return controller.CreateSystemdUserUnit(path, rootless.GetRootlessUID())
	}
	return controller.CreateSystemdUnit(path)
}

// deleteSystemdCgroup deletes the systemd cgroup at the given location
func deleteSystemdCgroup(path string) error {
	controller, err := cgroups.NewSystemd(getDefaultSystemdCgroup())
	if err != nil {
		return err
	}
	if rootless.IsRootless() {
		conn, err := cgroups.GetUserConnection(rootless.GetRootlessUID())
		if err != nil {
			return err
		}
		defer conn.Close()
		return controller.DeleteByPathConn(path, conn)
	}

	return controller.DeleteByPath(path)
}

// assembleSystemdCgroupName creates a systemd cgroup path given a base and
// a new component to add.
// The base MUST be systemd slice (end in .slice)
func assembleSystemdCgroupName(baseSlice, newSlice string) (string, error) {
	const sliceSuffix = ".slice"

	if !strings.HasSuffix(baseSlice, sliceSuffix) {
		return "", errors.Wrapf(define.ErrInvalidArg, "cannot assemble cgroup path with base %q - must end in .slice", baseSlice)
	}

	noSlice := strings.TrimSuffix(baseSlice, sliceSuffix)
	final := fmt.Sprintf("%s/%s-%s%s", baseSlice, noSlice, newSlice, sliceSuffix)

	return final, nil
}

var lvpRelabel = label.Relabel
var lvpInitLabels = label.InitLabels
var lvpReleaseLabel = label.ReleaseLabel

// LabelVolumePath takes a mount path for a volume and gives it an
// selinux label of either shared or not
func LabelVolumePath(path string) error {
	_, mountLabel, err := lvpInitLabels([]string{})
	if err != nil {
		return errors.Wrapf(err, "error getting default mountlabels")
	}
	if err := lvpReleaseLabel(mountLabel); err != nil {
		return errors.Wrapf(err, "error releasing label %q", mountLabel)
	}

	if err := lvpRelabel(path, mountLabel, true); err != nil {
		if err == syscall.ENOTSUP {
			logrus.Debugf("Labeling not supported on %q", path)
		} else {
			return errors.Wrapf(err, "error setting selinux label for %s to %q as shared", path, mountLabel)
		}
	}
	return nil
}

// Unmount umounts a target directory
func Unmount(mount string) {
	if err := unix.Unmount(mount, unix.MNT_DETACH); err != nil {
		if err != syscall.EINVAL {
			logrus.Warnf("failed to unmount %s : %v", mount, err)
		} else {
			logrus.Debugf("failed to unmount %s : %v", mount, err)
		}
	}
}
