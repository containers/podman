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
	"golang.org/x/sys/unix"
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
	src, dst, permissions, err := parseDevice(device)
	if err != nil {
		return err
	}
	dev, err := devices.DeviceFromPath(src, permissions)
	if err != nil {
		return errors.Wrapf(err, "%s is not a valid device", src)
	}
	dev.Path = dst
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

func (c *CreateConfig) addPrivilegedDevices(g *generate.Generator) error {
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

func (c *CreateConfig) createBlockIO() (*spec.LinuxBlockIO, error) {
	bio := &spec.LinuxBlockIO{}
	bio.Weight = &c.Resources.BlkioWeight
	if len(c.Resources.BlkioWeightDevice) > 0 {
		var lwds []spec.LinuxWeightDevice
		for _, i := range c.Resources.BlkioWeightDevice {
			wd, err := validateweightDevice(i)
			if err != nil {
				return bio, errors.Wrapf(err, "invalid values for blkio-weight-device")
			}
			wdStat, err := getStatFromPath(wd.path)
			if err != nil {
				return bio, errors.Wrapf(err, "error getting stat from path %q", wd.path)
			}
			lwd := spec.LinuxWeightDevice{
				Weight: &wd.weight,
			}
			lwd.Major = int64(unix.Major(wdStat.Rdev))
			lwd.Minor = int64(unix.Minor(wdStat.Rdev))
			lwds = append(lwds, lwd)
		}
		bio.WeightDevice = lwds
	}
	if len(c.Resources.DeviceReadBps) > 0 {
		readBps, err := makeThrottleArray(c.Resources.DeviceReadBps, bps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadBpsDevice = readBps
	}
	if len(c.Resources.DeviceWriteBps) > 0 {
		writeBpds, err := makeThrottleArray(c.Resources.DeviceWriteBps, bps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteBpsDevice = writeBpds
	}
	if len(c.Resources.DeviceReadIOps) > 0 {
		readIOps, err := makeThrottleArray(c.Resources.DeviceReadIOps, iops)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadIOPSDevice = readIOps
	}
	if len(c.Resources.DeviceWriteIOps) > 0 {
		writeIOps, err := makeThrottleArray(c.Resources.DeviceWriteIOps, iops)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteIOPSDevice = writeIOps
	}
	return bio, nil
}

func makeThrottleArray(throttleInput []string, rateType int) ([]spec.LinuxThrottleDevice, error) {
	var (
		ltds []spec.LinuxThrottleDevice
		t    *throttleDevice
		err  error
	)
	for _, i := range throttleInput {
		if rateType == bps {
			t, err = validateBpsDevice(i)
		} else {
			t, err = validateIOpsDevice(i)
		}
		if err != nil {
			return []spec.LinuxThrottleDevice{}, err
		}
		ltdStat, err := getStatFromPath(t.path)
		if err != nil {
			return ltds, errors.Wrapf(err, "error getting stat from path %q", t.path)
		}
		ltd := spec.LinuxThrottleDevice{
			Rate: t.rate,
		}
		ltd.Major = int64(unix.Major(ltdStat.Rdev))
		ltd.Minor = int64(unix.Minor(ltdStat.Rdev))
		ltds = append(ltds, ltd)
	}
	return ltds, nil
}
