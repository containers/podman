// +build linux darwin

package parse

import (
	"fmt"
	"os"
	"path/filepath"

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

func DeviceFromPath(device string) ([]configs.Device, error) {
	var devs []configs.Device
	src, dst, permissions, err := Device(device)
	if err != nil {
		return nil, err
	}
	if unshare.IsRootless() {
		return nil, errors.Errorf("Renaming device %s to %s is not a supported in rootless containers", src, dst)
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting info of source device %s", src)
	}

	if !srcInfo.IsDir() {

		dev, err := devices.DeviceFromPath(src, permissions)
		if err != nil {
			return nil, errors.Wrapf(err, "%s is not a valid device", src)
		}
		dev.Path = dst
		devs = append(devs, *dev)
		return devs, nil
	}

	// If source device is a directory
	srcDevices, err := devices.GetDevices(src)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting source devices from directory %s", src)
	}
	for _, d := range srcDevices {
		d.Path = filepath.Join(dst, filepath.Base(d.Path))
		d.Permissions = permissions
		devs = append(devs, *d)
	}
	return devs, nil
}
