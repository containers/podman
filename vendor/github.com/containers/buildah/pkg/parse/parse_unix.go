// +build linux darwin

package parse

import (
	"fmt"

	"github.com/containers/buildah/pkg/unshare"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func getDefaultProcessLimits() []string {
	rlim := unix.Rlimit{Cur: 1048576, Max: 1048576}
	defaultLimits := []string{}
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &rlim); err == nil {
		defaultLimits = append(defaultLimits, fmt.Sprintf("nofile=%d:%d", rlim.Cur, rlim.Max))
	}
	if err := unix.Setrlimit(unix.RLIMIT_NPROC, &rlim); err == nil {
		defaultLimits = append(defaultLimits, fmt.Sprintf("nproc=%d:%d", rlim.Cur, rlim.Max))
	}
	return defaultLimits
}

func DeviceFromPath(device string) (configs.Device, error) {
	src, dst, permissions, err := Device(device)
	if err != nil {
		return configs.Device{}, err
	}
	if unshare.IsRootless() {
		return configs.Device{}, errors.Errorf("Renaming device %s to %s is not a supported in rootless containers", src, dst)
	}
	dev, err := devices.DeviceFromPath(src, permissions)
	if err != nil {
		return configs.Device{}, errors.Wrapf(err, "%s is not a valid device", src)
	}
	dev.Path = dst
	return *dev, nil
}
