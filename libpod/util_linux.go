// +build linux

package libpod

import (
	"fmt"
	"strings"

	"github.com/containerd/cgroups"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

// makeSystemdCgroup creates a systemd CGroup at the given location.
func makeSystemdCgroup(path string) error {
	controller, err := cgroups.NewSystemd(SystemdDefaultCgroupParent)
	if err != nil {
		return err
	}

	return controller.Create(path, &spec.LinuxResources{})
}

// deleteSystemdCgroup deletes the systemd cgroup at the given location
func deleteSystemdCgroup(path string) error {
	controller, err := cgroups.NewSystemd(SystemdDefaultCgroupParent)
	if err != nil {
		return err
	}

	return controller.Delete(path)
}

// assembleSystemdCgroupName creates a systemd cgroup path given a base and
// a new component to add.
// The base MUST be systemd slice (end in .slice)
func assembleSystemdCgroupName(baseSlice, newSlice string) (string, error) {
	const sliceSuffix = ".slice"

	if !strings.HasSuffix(baseSlice, sliceSuffix) {
		return "", errors.Wrapf(ErrInvalidArg, "cannot assemble cgroup path with base %q - must end in .slice", baseSlice)
	}

	noSlice := strings.TrimSuffix(baseSlice, sliceSuffix)
	final := fmt.Sprintf("%s/%s-%s%s", baseSlice, noSlice, newSlice, sliceSuffix)

	return final, nil
}
