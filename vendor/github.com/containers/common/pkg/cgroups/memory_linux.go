//go:build linux

package cgroups

import (
	"fmt"
	"path/filepath"
	"strconv"

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

		path := filepath.Join(memoryRoot, "memory.stat")
		values, err := readCgroupMapPath(path)
		if err != nil {
			return err
		}

		// cgroup v1 does not have a single "anon" field, but we can calculate it
		// from total_active_anon and total_inactive_anon
		memUsage.Usage.Usage = 0
		for _, key := range []string{"total_active_anon", "total_inactive_anon"} {
			if _, found := values[key]; !found {
				continue
			}
			res, err := strconv.ParseUint(values[key][0], 10, 64)
			if err != nil {
				return fmt.Errorf("parse %s from %s: %w", key, path, err)
			}
			memUsage.Usage.Usage += res
		}
	}

	memUsage.Usage.Limit, err = readFileAsUint64(filepath.Join(memoryRoot, limitFilename))
	if err != nil {
		return err
	}

	m.MemoryStats = memUsage
	return nil
}
