//go:build linux

package cgroups

import (
	"path/filepath"
	"strconv"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs"
	"github.com/opencontainers/cgroups/fs2"
)

type linuxCPUHandler struct {
	CPU fs.CpuGroup
}

func getCPUHandler() *linuxCPUHandler {
	return &linuxCPUHandler{}
}

// Apply set the specified constraints.
func (c *linuxCPUHandler) Apply(ctr *CgroupControl, res *cgroups.Resources) error {
	man, err := fs2.NewManager(ctr.config, filepath.Join(cgroupRoot, ctr.config.Path))
	if err != nil {
		return err
	}
	return man.Set(res)
}

// Stat fills a metrics structure with usage stats for the controller.
func (c *linuxCPUHandler) Stat(ctr *CgroupControl, m *cgroups.Stats) error {
	cpu := cgroups.CpuStats{}
	values, err := readCgroup2MapFile(ctr, "cpu.stat")
	if err != nil {
		return err
	}
	if val, found := values["usage_usec"]; found {
		cpu.CpuUsage.TotalUsage, err = strconv.ParseUint(cleanString(val[0]), 10, 64)
		if err != nil {
			return err
		}
		cpu.CpuUsage.TotalUsage *= 1000
	}
	if val, found := values["system_usec"]; found {
		cpu.CpuUsage.UsageInKernelmode, err = strconv.ParseUint(cleanString(val[0]), 10, 64)
		if err != nil {
			return err
		}
		cpu.CpuUsage.UsageInKernelmode *= 1000
	}
	m.CpuStats = cpu
	return nil
}
