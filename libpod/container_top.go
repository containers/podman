package libpod

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/utils"
	"github.com/sirupsen/logrus"
)

// GetContainerPids reads sysfs to obtain the pids associated with the container's cgroup
// and uses locking
func (c *Container) GetContainerPids() ([]string, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return []string{}, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.getContainerPids()
}

// Gets the pids for a container without locking.  should only be called from a func where
// locking has already been established.
func (c *Container) getContainerPids() ([]string, error) {
	taskFile := filepath.Join("/sys/fs/cgroup/pids", c.config.CgroupParent, fmt.Sprintf("libpod-conmon-%s", c.ID()), c.ID(), "tasks")
	logrus.Debug("reading pids from ", taskFile)
	content, err := ioutil.ReadFile(taskFile)
	if err != nil {
		return []string{}, errors.Wrapf(err, "unable to read pids from %s", taskFile)
	}
	return strings.Fields(string(content)), nil

}

// GetContainerPidInformation calls ps with the appropriate options and returns
// the results as a string
func (c *Container) GetContainerPidInformation(args []string) ([]string, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return []string{}, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	pids, err := c.getContainerPids()
	if err != nil {
		return []string{}, errors.Wrapf(err, "unable to obtain pids for ", c.ID())
	}
	args = append(args, "-p", strings.Join(pids, ","))
	logrus.Debug("Executing: ", strings.Join(args, " "))
	results, err := utils.ExecCmd("ps", args...)
	if err != nil {
		return []string{}, errors.Wrapf(err, "unable to obtain information about pids")
	}
	return strings.Split(results, "\n"), nil
}
