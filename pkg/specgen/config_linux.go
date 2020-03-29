package specgen

//func createBlockIO() (*spec.LinuxBlockIO, error) {
//	var ret *spec.LinuxBlockIO
//	bio := &spec.LinuxBlockIO{}
//	if c.Resources.BlkioWeight > 0 {
//		ret = bio
//		bio.Weight = &c.Resources.BlkioWeight
//	}
//	if len(c.Resources.BlkioWeightDevice) > 0 {
//		var lwds []spec.LinuxWeightDevice
//		ret = bio
//		for _, i := range c.Resources.BlkioWeightDevice {
//			wd, err := ValidateweightDevice(i)
//			if err != nil {
//				return ret, errors.Wrapf(err, "invalid values for blkio-weight-device")
//			}
//			wdStat, err := GetStatFromPath(wd.Path)
//			if err != nil {
//				return ret, errors.Wrapf(err, "error getting stat from path %q", wd.Path)
//			}
//			lwd := spec.LinuxWeightDevice{
//				Weight: &wd.Weight,
//			}
//			lwd.Major = int64(unix.Major(wdStat.Rdev))
//			lwd.Minor = int64(unix.Minor(wdStat.Rdev))
//			lwds = append(lwds, lwd)
//		}
//		bio.WeightDevice = lwds
//	}
//	if len(c.Resources.DeviceReadBps) > 0 {
//		ret = bio
//		readBps, err := makeThrottleArray(c.Resources.DeviceReadBps, bps)
//		if err != nil {
//			return ret, err
//		}
//		bio.ThrottleReadBpsDevice = readBps
//	}
//	if len(c.Resources.DeviceWriteBps) > 0 {
//		ret = bio
//		writeBpds, err := makeThrottleArray(c.Resources.DeviceWriteBps, bps)
//		if err != nil {
//			return ret, err
//		}
//		bio.ThrottleWriteBpsDevice = writeBpds
//	}
//	if len(c.Resources.DeviceReadIOps) > 0 {
//		ret = bio
//		readIOps, err := makeThrottleArray(c.Resources.DeviceReadIOps, iops)
//		if err != nil {
//			return ret, err
//		}
//		bio.ThrottleReadIOPSDevice = readIOps
//	}
//	if len(c.Resources.DeviceWriteIOps) > 0 {
//		ret = bio
//		writeIOps, err := makeThrottleArray(c.Resources.DeviceWriteIOps, iops)
//		if err != nil {
//			return ret, err
//		}
//		bio.ThrottleWriteIOPSDevice = writeIOps
//	}
//	return ret, nil
//}

//func makeThrottleArray(throttleInput []string, rateType int) ([]spec.LinuxThrottleDevice, error) {
//	var (
//		ltds []spec.LinuxThrottleDevice
//		t    *throttleDevice
//		err  error
//	)
//	for _, i := range throttleInput {
//		if rateType == bps {
//			t, err = validateBpsDevice(i)
//		} else {
//			t, err = validateIOpsDevice(i)
//		}
//		if err != nil {
//			return []spec.LinuxThrottleDevice{}, err
//		}
//		ltdStat, err := GetStatFromPath(t.path)
//		if err != nil {
//			return ltds, errors.Wrapf(err, "error getting stat from path %q", t.path)
//		}
//		ltd := spec.LinuxThrottleDevice{
//			Rate: t.rate,
//		}
//		ltd.Major = int64(unix.Major(ltdStat.Rdev))
//		ltd.Minor = int64(unix.Minor(ltdStat.Rdev))
//		ltds = append(ltds, ltd)
//	}
//	return ltds, nil
//}
