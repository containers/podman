//go:build linux
// +build linux

package libpod

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// systemdSliceFromPath makes a new systemd slice under the given parent with
// the given name.
// The parent must be a slice. The name must NOT include ".slice"
func systemdSliceFromPath(parent, name string, resources *spec.LinuxResources) (string, error) {
	cgroupPath, err := assembleSystemdCgroupName(parent, name)
	if err != nil {
		return "", err
	}

	logrus.Debugf("Created cgroup path %s for parent %s and name %s", cgroupPath, parent, name)

	if err := makeSystemdCgroup(cgroupPath, resources); err != nil {
		return "", fmt.Errorf("creating cgroup %s: %w", cgroupPath, err)
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

// makeSystemdCgroup creates a systemd Cgroup at the given location.
func makeSystemdCgroup(path string, resources *spec.LinuxResources) error {
	res, err := GetLimits(resources)
	if err != nil {
		return err
	}
	controller, err := cgroups.NewSystemd(getDefaultSystemdCgroup(), &res)
	if err != nil {
		return err
	}

	if rootless.IsRootless() {
		return controller.CreateSystemdUserUnit(path, rootless.GetRootlessUID())
	}
	err = controller.CreateSystemdUnit(path)
	if err != nil {
		return err
	}
	return nil
}

// deleteSystemdCgroup deletes the systemd cgroup at the given location
func deleteSystemdCgroup(path string, resources *spec.LinuxResources) error {
	res, err := GetLimits(resources)
	if err != nil {
		return err
	}
	controller, err := cgroups.NewSystemd(getDefaultSystemdCgroup(), &res)
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
		return "", fmt.Errorf("cannot assemble cgroup path with base %q - must end in .slice: %w", baseSlice, define.ErrInvalidArg)
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
func LabelVolumePath(path, mountLabel string) error {
	if mountLabel == "" {
		var err error
		_, mountLabel, err = lvpInitLabels([]string{})
		if err != nil {
			return fmt.Errorf("getting default mountlabels: %w", err)
		}
		if err := lvpReleaseLabel(mountLabel); err != nil {
			return fmt.Errorf("releasing label %q: %w", mountLabel, err)
		}
	}

	if err := lvpRelabel(path, mountLabel, true); err != nil {
		if err == syscall.ENOTSUP {
			logrus.Debugf("Labeling not supported on %q", path)
		} else {
			return fmt.Errorf("setting selinux label for %s to %q as shared: %w", path, mountLabel, err)
		}
	}
	return nil
}

// Unmount umounts a target directory
func Unmount(mount string) {
	if err := unix.Unmount(mount, unix.MNT_DETACH); err != nil {
		if err != syscall.EINVAL {
			logrus.Warnf("Failed to unmount %s : %v", mount, err)
		} else {
			logrus.Debugf("failed to unmount %s : %v", mount, err)
		}
	}
}
