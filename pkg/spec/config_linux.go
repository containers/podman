// +build linux

package createconfig

import (
	"io/ioutil"

	"github.com/docker/docker/profiles/seccomp"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
)

// Device transforms a libcontainer configs.Device to a specs.LinuxDevice object.
func Device(d *configs.Device) spec.LinuxDevice {
	return spec.LinuxDevice{
		Type:     string(d.Type),
		Path:     d.Path,
		Major:    d.Major,
		Minor:    d.Minor,
		FileMode: fmPtr(int64(d.FileMode)),
		UID:      u32Ptr(int64(d.Uid)),
		GID:      u32Ptr(int64(d.Gid)),
	}
}

func addDevice(g *generate.Generator, device string) error {
	dev, err := devices.DeviceFromPath(device, "rwm")
	if err != nil {
		return errors.Wrapf(err, "%s is not a valid device", device)
	}
	linuxdev := spec.LinuxDevice{
		Path:     dev.Path,
		Type:     string(dev.Type),
		Major:    dev.Major,
		Minor:    dev.Minor,
		FileMode: &dev.FileMode,
		UID:      &dev.Uid,
		GID:      &dev.Gid,
	}
	g.AddDevice(linuxdev)
	g.AddLinuxResourcesDevice(true, string(dev.Type), &dev.Major, &dev.Minor, dev.Permissions)
	return nil
}

// AddPrivilegedDevices iterates through host devices and adds all
// host devices to the spec
func (c *CreateConfig) AddPrivilegedDevices(g *generate.Generator) error {
	hostDevices, err := devices.HostDevices()
	if err != nil {
		return err
	}
	g.ClearLinuxDevices()
	for _, d := range hostDevices {
		g.AddDevice(Device(d))
	}
	g.AddLinuxResourcesDevice(true, "", nil, nil, "rwm")
	return nil
}

func getSeccompConfig(config *CreateConfig, configSpec *spec.Spec) (*spec.LinuxSeccomp, error) {
	var seccompConfig *spec.LinuxSeccomp
	var err error

	if config.SeccompProfilePath != "" {
		seccompProfile, err := ioutil.ReadFile(config.SeccompProfilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "opening seccomp profile (%s) failed", config.SeccompProfilePath)
		}
		seccompConfig, err = seccomp.LoadProfile(string(seccompProfile), configSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
		}
	} else {
		seccompConfig, err = seccomp.GetDefaultProfile(configSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
		}
	}

	return seccompConfig, nil
}
