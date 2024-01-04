//go:build linux

package specgen

import (
	"fmt"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

// FinishThrottleDevices takes the temporary representation of the throttle
// devices in the specgen and looks up the major and major minors. it then
// sets the throttle devices proper in the specgen
func FinishThrottleDevices(s *SpecGenerator) error {
	if s.ResourceLimits == nil {
		s.ResourceLimits = &spec.LinuxResources{}
	}
	if bps := s.ThrottleReadBpsDevice; len(bps) > 0 {
		if s.ResourceLimits.BlockIO == nil {
			s.ResourceLimits.BlockIO = &spec.LinuxBlockIO{}
		}
		for k, v := range bps {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return fmt.Errorf("could not parse throttle device at %s: %w", k, err)
			}
			v.Major = (int64(unix.Major(uint64(statT.Rdev)))) //nolint: unconvert
			v.Minor = (int64(unix.Minor(uint64(statT.Rdev)))) //nolint: unconvert
			if s.ResourceLimits.BlockIO == nil {
				s.ResourceLimits.BlockIO = new(spec.LinuxBlockIO)
			}
			s.ResourceLimits.BlockIO.ThrottleReadBpsDevice = append(s.ResourceLimits.BlockIO.ThrottleReadBpsDevice, v)
		}
	}
	if bps := s.ThrottleWriteBpsDevice; len(bps) > 0 {
		if s.ResourceLimits.BlockIO == nil {
			s.ResourceLimits.BlockIO = &spec.LinuxBlockIO{}
		}
		for k, v := range bps {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return fmt.Errorf("could not parse throttle device at %s: %w", k, err)
			}
			v.Major = (int64(unix.Major(uint64(statT.Rdev)))) //nolint: unconvert
			v.Minor = (int64(unix.Minor(uint64(statT.Rdev)))) //nolint: unconvert
			s.ResourceLimits.BlockIO.ThrottleWriteBpsDevice = append(s.ResourceLimits.BlockIO.ThrottleWriteBpsDevice, v)
		}
	}
	if iops := s.ThrottleReadIOPSDevice; len(iops) > 0 {
		if s.ResourceLimits.BlockIO == nil {
			s.ResourceLimits.BlockIO = &spec.LinuxBlockIO{}
		}
		for k, v := range iops {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return fmt.Errorf("could not parse throttle device at %s: %w", k, err)
			}
			v.Major = (int64(unix.Major(uint64(statT.Rdev)))) //nolint: unconvert
			v.Minor = (int64(unix.Minor(uint64(statT.Rdev)))) //nolint: unconvert
			s.ResourceLimits.BlockIO.ThrottleReadIOPSDevice = append(s.ResourceLimits.BlockIO.ThrottleReadIOPSDevice, v)
		}
	}
	if iops := s.ThrottleWriteIOPSDevice; len(iops) > 0 {
		if s.ResourceLimits.BlockIO == nil {
			s.ResourceLimits.BlockIO = &spec.LinuxBlockIO{}
		}
		for k, v := range iops {
			statT := unix.Stat_t{}
			if err := unix.Stat(k, &statT); err != nil {
				return fmt.Errorf("could not parse throttle device at %s: %w", k, err)
			}
			v.Major = (int64(unix.Major(uint64(statT.Rdev)))) //nolint: unconvert
			v.Minor = (int64(unix.Minor(uint64(statT.Rdev)))) //nolint: unconvert
			s.ResourceLimits.BlockIO.ThrottleWriteIOPSDevice = append(s.ResourceLimits.BlockIO.ThrottleWriteIOPSDevice, v)
		}
	}
	return nil
}

func WeightDevices(specgen *SpecGenerator) error {
	devs := []spec.LinuxWeightDevice{}
	if specgen.ResourceLimits == nil {
		specgen.ResourceLimits = &spec.LinuxResources{}
	}
	for k, v := range specgen.WeightDevice {
		statT := unix.Stat_t{}
		if err := unix.Stat(k, &statT); err != nil {
			return fmt.Errorf("failed to inspect '%s' in --blkio-weight-device: %w", k, err)
		}
		dev := new(spec.LinuxWeightDevice)
		dev.Major = (int64(unix.Major(uint64(statT.Rdev)))) //nolint: unconvert
		dev.Minor = (int64(unix.Minor(uint64(statT.Rdev)))) //nolint: unconvert
		dev.Weight = v.Weight
		devs = append(devs, *dev)
		if specgen.ResourceLimits.BlockIO == nil {
			specgen.ResourceLimits.BlockIO = &spec.LinuxBlockIO{}
		}
		specgen.ResourceLimits.BlockIO.WeightDevice = devs
	}
	return nil
}
