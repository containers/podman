//go:build !linux
// +build !linux

package cgroups

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

type cpuHandler struct{}

func getCPUHandler() *cpuHandler {
	return &cpuHandler{}
}

// Apply set the specified constraints
func (c *cpuHandler) Apply(ctr *CgroupControl, res *spec.LinuxResources) error {
	if res.CPU == nil {
		return nil
	}
	return fmt.Errorf("cpu apply not implemented yet")
}

// Create the cgroup
func (c *cpuHandler) Create(ctr *CgroupControl) (bool, error) {
	if ctr.cgroup2 {
		return false, nil
	}
	return ctr.createCgroupDirectory(CPU)
}

// Destroy the cgroup
func (c *cpuHandler) Destroy(ctr *CgroupControl) error {
	return rmDirRecursively(ctr.getCgroupv1Path(CPU))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *cpuHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	var err error
	usage := CPUUsage{}
	if ctr.cgroup2 {
		values, err := readCgroup2MapFile(ctr, "cpu.stat")
		if err != nil {
			return err
		}
		if val, found := values["usage_usec"]; found {
			usage.Total, err = strconv.ParseUint(cleanString(val[0]), 10, 64)
			if err != nil {
				return err
			}
			usage.Kernel *= 1000
		}
		if val, found := values["system_usec"]; found {
			usage.Kernel, err = strconv.ParseUint(cleanString(val[0]), 10, 64)
			if err != nil {
				return err
			}
			usage.Total *= 1000
		}
		// FIXME: How to read usage.PerCPU?
	} else {
		usage.Total, err = readAcct(ctr, "cpuacct.usage")
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			usage.Total = 0
		}
		usage.Kernel, err = readAcct(ctr, "cpuacct.usage_sys")
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			usage.Kernel = 0
		}
		usage.PerCPU, err = readAcctList(ctr, "cpuacct.usage_percpu")
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			usage.PerCPU = nil
		}
	}
	m.CPU = CPUMetrics{Usage: usage}
	return nil
}
