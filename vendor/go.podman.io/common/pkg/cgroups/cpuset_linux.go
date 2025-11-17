//go:build linux

package cgroups

import (
	"path/filepath"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs"
	"github.com/opencontainers/cgroups/fs2"
)

type linuxCpusetHandler struct {
	CPUSet fs.CpusetGroup
}

func getCpusetHandler() *linuxCpusetHandler {
	return &linuxCpusetHandler{}
}

// Apply set the specified constraints.
func (c *linuxCpusetHandler) Apply(ctr *CgroupControl, res *cgroups.Resources) error {
	man, err := fs2.NewManager(ctr.config, filepath.Join(cgroupRoot, ctr.config.Path))
	if err != nil {
		return err
	}
	return man.Set(res)
}

// Stat fills a metrics structure with usage stats for the controller.
func (c *linuxCpusetHandler) Stat(_ *CgroupControl, _ *cgroups.Stats) error {
	return nil
}
