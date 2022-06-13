//go:build linux
// +build linux

package cgroups

import (
	"path/filepath"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type linuxMemHandler struct {
	Mem fs.MemoryGroup
}

func getMemoryHandler() *linuxMemHandler {
	return &linuxMemHandler{}
}

// Apply set the specified constraints
func (c *linuxMemHandler) Apply(ctr *CgroupControl, res *configs.Resources) error {
	if ctr.cgroup2 {
		man, err := fs2.NewManager(ctr.config, filepath.Join(cgroupRoot, ctr.config.Path))
		if err != nil {
			return err
		}
		return man.Set(res)
	}
	path := filepath.Join(cgroupRoot, Memory, ctr.config.Path)
	return c.Mem.Set(path, res)
}

// Create the cgroup
func (c *linuxMemHandler) Create(ctr *CgroupControl) (bool, error) {
	if ctr.cgroup2 {
		return false, nil
	}
	return ctr.createCgroupDirectory(Memory)
}

// Destroy the cgroup
func (c *linuxMemHandler) Destroy(ctr *CgroupControl) error {
	return rmDirRecursively(ctr.getCgroupv1Path(Memory))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *linuxMemHandler) Stat(ctr *CgroupControl, m *cgroups.Stats) error {
	var err error
	memUsage := cgroups.MemoryStats{}

	var memoryRoot string
	var limitFilename string

	if ctr.cgroup2 {
		memoryRoot = filepath.Join(cgroupRoot, ctr.config.Path)
		limitFilename = "memory.max"
		if memUsage.Usage.Usage, err = readFileByKeyAsUint64(filepath.Join(memoryRoot, "memory.stat"), "anon"); err != nil {
			return err
		}
	} else {
		memoryRoot = ctr.getCgroupv1Path(Memory)
		limitFilename = "memory.limit_in_bytes"
		if memUsage.Usage.Usage, err = readFileAsUint64(filepath.Join(memoryRoot, "memory.usage_in_bytes")); err != nil {
			return err
		}
	}

	memUsage.Usage.Limit, err = readFileAsUint64(filepath.Join(memoryRoot, limitFilename))
	if err != nil {
		return err
	}

	m.MemoryStats = memUsage
	return nil
}
