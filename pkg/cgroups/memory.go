package cgroups

import (
	"fmt"
	"path/filepath"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

type memHandler struct {
}

func getMemoryHandler() *memHandler {
	return &memHandler{}
}

// Apply set the specified constraints
func (c *memHandler) Apply(ctr *CgroupControl, res *spec.LinuxResources) error {
	if res.Memory == nil {
		return nil
	}
	return fmt.Errorf("memory apply not implemented yet")
}

// Create the cgroup
func (c *memHandler) Create(ctr *CgroupControl) (bool, error) {
	if ctr.cgroup2 {
		return false, nil
	}
	return ctr.createCgroupDirectory(Memory)
}

// Destroy the cgroup
func (c *memHandler) Destroy(ctr *CgroupControl) error {
	return rmDirRecursively(ctr.getCgroupv1Path(Memory))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *memHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	var err error
	usage := MemoryUsage{}

	var memoryRoot string
	filenames := map[string]string{}

	if ctr.cgroup2 {
		memoryRoot = filepath.Join(cgroupRoot, ctr.path)
		filenames["usage"] = "memory.current"
		filenames["limit"] = "memory.max"
	} else {
		memoryRoot = ctr.getCgroupv1Path(Memory)
		filenames["usage"] = "memory.usage_in_bytes"
		filenames["limit"] = "memory.limit_in_bytes"
	}
	usage.Usage, err = readFileAsUint64(filepath.Join(memoryRoot, filenames["usage"]))
	if err != nil {
		return err
	}
	usage.Limit, err = readFileAsUint64(filepath.Join(memoryRoot, filenames["limit"]))
	if err != nil {
		return err
	}

	m.Memory = MemoryMetrics{Usage: usage}
	return nil
}
