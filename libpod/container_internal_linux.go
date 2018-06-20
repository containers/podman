// +build linux

package libpod

import (
	"fmt"
	"path/filepath"

	"github.com/containerd/cgroups"
	"github.com/sirupsen/logrus"
)

// cleanupCgroup cleans up residual CGroups after container execution
// This is a no-op for the systemd cgroup driver
func (c *Container) cleanupCgroups() error {
	if !c.state.CgroupCreated {
		logrus.Debugf("Cgroups are not present, ignoring...")
		return nil
	}

	if c.runtime.config.CgroupManager == SystemdCgroupsManager {
		return nil
	}

	// Remove the base path of the container's cgroups
	path := filepath.Join(c.config.CgroupParent, fmt.Sprintf("libpod-%s", c.ID()))

	logrus.Debugf("Removing CGroup %s", path)

	cgroup, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(path))
	if err != nil {
		// It's fine for the cgroup to not exist
		// We want it gone, it's gone
		if err == cgroups.ErrCgroupDeleted {
			return nil
		}

		return err
	}

	if err := cgroup.Delete(); err != nil {
		return err
	}

	c.state.CgroupCreated = false

	if c.valid {
		return c.save()
	}

	return nil
}
