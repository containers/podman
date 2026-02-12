//go:build linux
// +build linux

package cgroups

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type BlkioDev struct {
	Device string
	Bytes  uint64
}

func systemdCreate(resources *configs.Resources, path string, c *systemdDbus.Conn) error {
	slice, name := filepath.Split(path)
	slice = strings.TrimSuffix(slice, "/")

	var lastError error
	for i := 0; i < 2; i++ {
		properties := []systemdDbus.Property{
			systemdDbus.PropDescription(fmt.Sprintf("cgroup %s", name)),
			systemdDbus.PropWants(slice),
		}
		var ioString string
		v2, _ := IsCgroup2UnifiedMode()
		if v2 {
			ioString = "IOAccounting"
		} else {
			ioString = "BlockIOAccounting"
		}
		pMap := map[string]bool{
			"DefaultDependencies": false,
			"MemoryAccounting":    true,
			"CPUAccounting":       true,
			ioString:              true,
		}
		if i == 0 {
			pMap["Delegate"] = true
		}

		for k, v := range pMap {
			p := systemdDbus.Property{
				Name:  k,
				Value: dbus.MakeVariant(v),
			}
			properties = append(properties, p)
		}

		uMap, sMap, bMap, iMap, structMap := resourcesToProps(resources, v2)
		for k, v := range uMap {
			p := systemdDbus.Property{
				Name:  k,
				Value: dbus.MakeVariant(v),
			}
			properties = append(properties, p)
		}

		for k, v := range sMap {
			p := systemdDbus.Property{
				Name:  k,
				Value: dbus.MakeVariant(v),
			}
			properties = append(properties, p)
		}

		for k, v := range bMap {
			p := systemdDbus.Property{
				Name:  k,
				Value: dbus.MakeVariant(v),
			}
			properties = append(properties, p)
		}

		for k, v := range iMap {
			p := systemdDbus.Property{
				Name:  k,
				Value: dbus.MakeVariant(v),
			}
			properties = append(properties, p)
		}

		for k, v := range structMap {
			p := systemdDbus.Property{
				Name:  k,
				Value: dbus.MakeVariant(v),
			}
			properties = append(properties, p)
		}

		ch := make(chan string)
		_, err := c.StartTransientUnitContext(context.TODO(), name, "replace", properties, ch)
		if err != nil {
			lastError = err
			continue
		}
		<-ch
		return nil
	}
	return lastError
}

/*
systemdDestroyConn is copied from containerd/cgroups/systemd.go file, that
has the following license:

Copyright The containerd Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
func systemdDestroyConn(path string, c *systemdDbus.Conn) error {
	name := filepath.Base(path)

	ch := make(chan string)
	_, err := c.StopUnitContext(context.TODO(), name, "replace", ch)
	if err != nil {
		return err
	}
	<-ch
	return nil
}

func resourcesToProps(res *configs.Resources, v2 bool) (map[string]uint64, map[string]string, map[string][]byte, map[string]int64, map[string][]BlkioDev) {
	bMap := make(map[string][]byte)
	// this array is not used but will be once more resource limits are added
	sMap := make(map[string]string)
	iMap := make(map[string]int64)
	uMap := make(map[string]uint64)
	structMap := make(map[string][]BlkioDev)

	// CPU
	if res.CpuPeriod != 0 {
		uMap["CPUQuotaPeriodUSec"] = res.CpuPeriod
	}
	if res.CpuQuota != 0 {
		period := res.CpuPeriod
		if period == 0 {
			period = uint64(100000)
		}
		cpuQuotaPerSecUSec := uint64(res.CpuQuota*1000000) / period
		if cpuQuotaPerSecUSec%10000 != 0 {
			cpuQuotaPerSecUSec = ((cpuQuotaPerSecUSec / 10000) + 1) * 10000
		}
		uMap["CPUQuotaPerSecUSec"] = cpuQuotaPerSecUSec
	}

	if res.CpuShares != 0 {
		// convert from shares to weight. weight only supports 1-10000
		v2, _ := IsCgroup2UnifiedMode()
		if v2 {
			wt := (1 + ((res.CpuShares-2)*9999)/262142)
			uMap["CPUWeight"] = wt
		} else {
			uMap["CPUShares"] = res.CpuShares
		}
	}

	// CPUSet
	if res.CpusetCpus != "" {
		bits := []byte(res.CpusetCpus)
		bMap["AllowedCPUs"] = bits
	}
	if res.CpusetMems != "" {
		bits := []byte(res.CpusetMems)
		bMap["AllowedMemoryNodes"] = bits
	}

	// Mem
	if res.Memory != 0 {
		uMap["MemoryMax"] = uint64(res.Memory)
	}
	if res.MemorySwap != 0 {
		switch {
		case res.Memory == -1 || res.MemorySwap == -1:
			swap := -1
			uMap["MemorySwapMax"] = uint64(swap)
		case v2:
			// swap max = swap (limit + swap limit) - limit
			uMap["MemorySwapMax"] = uint64(res.MemorySwap - res.Memory)
		default:
			uMap["MemorySwapMax"] = uint64(res.MemorySwap)
		}
	}

	// Blkio
	if res.BlkioWeight > 0 {
		if v2 {
			uMap["IOWeight"] = uint64(res.BlkioWeight)
		} else {
			uMap["BlockIOWeight"] = uint64(res.BlkioWeight)
		}
	}

	// systemd requires the paths to be in the form /dev/{block, char}/major:minor
	// this is difficult since runc's resources only store the major and minor, not the type of device
	// therefore, assume all are block (I think this is a correct assumption)
	if res.BlkioThrottleReadBpsDevice != nil {
		for _, entry := range res.BlkioThrottleReadBpsDevice {
			newThrottle := BlkioDev{
				Device: fmt.Sprintf("/dev/block/%d:%d", entry.Major, entry.Minor),
				Bytes:  entry.Rate,
			}
			if v2 {
				structMap["IOReadBandwidthMax"] = append(structMap["IOReadBandwidthMax"], newThrottle)
			} else {
				structMap["BlockIOReadBandwidth"] = append(structMap["BlockIOReadBandwidth"], newThrottle)
			}
		}
	}

	if res.BlkioThrottleWriteBpsDevice != nil {
		for _, entry := range res.BlkioThrottleWriteBpsDevice {
			newThrottle := BlkioDev{
				Device: fmt.Sprintf("/dev/block/%d:%d", entry.Major, entry.Minor),
				Bytes:  entry.Rate,
			}
			if v2 {
				structMap["IOWriteBandwidthMax"] = append(structMap["IOWriteBandwidthMax"], newThrottle)
			} else {
				structMap["BlockIOWriteBandwidth"] = append(structMap["BlockIOWriteBandwidth"], newThrottle)
			}
		}
	}

	if res.BlkioWeightDevice != nil {
		for _, entry := range res.BlkioWeightDevice {
			newWeight := BlkioDev{
				Device: fmt.Sprintf("/dev/block/%d:%d", entry.Major, entry.Minor),
				Bytes:  uint64(entry.Weight),
			}
			if v2 {
				structMap["IODeviceWeight"] = append(structMap["IODeviceWeight"], newWeight)
			} else {
				structMap["BlockIODeviceWeight"] = append(structMap["BlockIODeviceWeight"], newWeight)
			}
		}
	}

	return uMap, sMap, bMap, iMap, structMap
}
