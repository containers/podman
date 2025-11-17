//go:build linux

package cgroups

import (
	"path/filepath"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs"
	"github.com/opencontainers/cgroups/fs2"
)

type linuxPidHandler struct {
	Pid fs.PidsGroup
}

func getPidsHandler() *linuxPidHandler {
	return &linuxPidHandler{}
}

// Apply set the specified constraints.
func (c *linuxPidHandler) Apply(ctr *CgroupControl, res *cgroups.Resources) error {
	man, err := fs2.NewManager(ctr.config, filepath.Join(cgroupRoot, ctr.config.Path))
	if err != nil {
		return err
	}
	return man.Set(res)
}

// Stat fills a metrics structure with usage stats for the controller.
func (c *linuxPidHandler) Stat(ctr *CgroupControl, m *cgroups.Stats) error {
	if ctr.config.Path == "" {
		// nothing we can do to retrieve the pids.current path
		return nil
	}

	PIDRoot := filepath.Join(cgroupRoot, ctr.config.Path)

	current, err := readFileAsUint64(filepath.Join(PIDRoot, "pids.current"))
	if err != nil {
		return err
	}

	m.PidsStats.Current = current
	return nil
}
