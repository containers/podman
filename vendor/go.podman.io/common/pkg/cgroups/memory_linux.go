//go:build linux

package cgroups

import (
	"path/filepath"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs"
	"github.com/opencontainers/cgroups/fs2"
)

type linuxMemHandler struct {
	Mem fs.MemoryGroup
}

func getMemoryHandler() *linuxMemHandler {
	return &linuxMemHandler{}
}

// Apply set the specified constraints.
func (c *linuxMemHandler) Apply(ctr *CgroupControl, res *cgroups.Resources) error {
	man, err := fs2.NewManager(ctr.config, filepath.Join(cgroupRoot, ctr.config.Path))
	if err != nil {
		return err
	}
	return man.Set(res)
}

// Create the cgroup.
func (c *linuxMemHandler) Create(ctr *CgroupControl) (bool, error) {
	return false, nil
}

// Destroy the cgroup.
func (c *linuxMemHandler) Destroy(ctr *CgroupControl) error {
	return rmDirRecursively(ctr.getCgroupv1Path(Memory))
}

// Stat fills a metrics structure with usage stats for the controller.
func (c *linuxMemHandler) Stat(ctr *CgroupControl, m *cgroups.Stats) error {
	var err error
	memUsage := cgroups.MemoryStats{}

	var memoryRoot string
	var limitFilename string

	memoryRoot = filepath.Join(cgroupRoot, ctr.config.Path)
	limitFilename = "memory.max"

	// Read memory.current
	current, err := readFileAsUint64(filepath.Join(memoryRoot, "memory.current"))
	if err != nil {
		return err
	}

	// Read inactive_file from memory.stat
	inactiveFile, err := readFileByKeyAsUint64(filepath.Join(memoryRoot, "memory.stat"), "inactive_file")
	if err != nil {
		return err
	}

	// Docker calculation: memory.current - memory.stat['inactive_file']
	memUsage.Usage.Usage = 0
	if inactiveFile < current {
		memUsage.Usage.Usage = current - inactiveFile
	}

	memUsage.Usage.Limit, err = readFileAsUint64(filepath.Join(memoryRoot, limitFilename))
	if err != nil {
		return err
	}

	m.MemoryStats = memUsage
	return nil
}
