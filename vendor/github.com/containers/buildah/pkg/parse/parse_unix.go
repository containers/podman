// +build linux darwin

package parse

import (
	"os"
	"path/filepath"

	"github.com/containers/buildah/define"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/pkg/errors"
)

func DeviceFromPath(device string) (define.ContainerDevices, error) {
	var devs define.ContainerDevices
	src, dst, permissions, err := Device(device)
	if err != nil {
		return nil, err
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
		device := define.BuildahDevice{Device: *dev, Source: src, Destination: dst}
		devs = append(devs, device)
		return devs, nil
	}

	// If source device is a directory
	srcDevices, err := devices.GetDevices(src)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting source devices from directory %s", src)
	}
	for _, d := range srcDevices {
		d.Path = filepath.Join(dst, filepath.Base(d.Path))
		d.Permissions = devices.Permissions(permissions)
		device := define.BuildahDevice{Device: *d, Source: src, Destination: dst}
		devs = append(devs, device)
	}
	return devs, nil
}
