package cgroups

import (
	"fmt"
	"os"
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
	return fmt.Errorf("function not implemented yet")
}

// Create the cgroup
func (c *memHandler) Create(ctr *CgroupControl) (bool, error) {
	if ctr.cgroup2 {
		return false, fmt.Errorf("function not implemented yet")
	}
	return ctr.createCgroupDirectory(Memory)
}

// Destroy the cgroup
func (c *memHandler) Destroy(ctr *CgroupControl) error {
	return os.Remove(ctr.getCgroupv1Path(Memory))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *memHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	if ctr.cgroup2 {
		return fmt.Errorf("function not implemented yet")
	}
	usage := MemoryUsage{}

	memoryRoot := ctr.getCgroupv1Path(Memory)

	var err error
	usage.Usage, err = readFileAsUint64(filepath.Join(memoryRoot, "memory.usage_in_bytes"))
	if err != nil {
		return err
	}
	usage.Limit, err = readFileAsUint64(filepath.Join(memoryRoot, "memory.limit_in_bytes"))
	if err != nil {
		return err
	}

	m.Memory = MemoryMetrics{Usage: usage}
	return nil
}
