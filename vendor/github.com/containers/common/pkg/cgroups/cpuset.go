//go:build !linux
// +build !linux

package cgroups

import (
	"fmt"
	"path/filepath"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

type cpusetHandler struct{}

func getCpusetHandler() *cpusetHandler {
	return &cpusetHandler{}
}

// Apply set the specified constraints
func (c *cpusetHandler) Apply(ctr *CgroupControl, res *spec.LinuxResources) error {
	if res.CPU == nil {
		return nil
	}
	return fmt.Errorf("cpuset apply not implemented yet")
}

// Create the cgroup
func (c *cpusetHandler) Create(ctr *CgroupControl) (bool, error) {
	if ctr.cgroup2 {
		path := filepath.Join(cgroupRoot, ctr.path)
		return true, cpusetCopyFromParent(path, true)
	}

	created, err := ctr.createCgroupDirectory(CPUset)
	if !created || err != nil {
		return created, err
	}
	return true, cpusetCopyFromParent(ctr.getCgroupv1Path(CPUset), false)
}

// Destroy the cgroup
func (c *cpusetHandler) Destroy(ctr *CgroupControl) error {
	return rmDirRecursively(ctr.getCgroupv1Path(CPUset))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *cpusetHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	return nil
}
