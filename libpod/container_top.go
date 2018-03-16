package libpod

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/projectatomic/libpod/utils"
	"github.com/sirupsen/logrus"
)

// GetContainerPids reads sysfs to obtain the pids associated with the container's cgroup
// and uses locking
func (c *Container) GetContainerPids() ([]string, error) {
	if !c.batched {
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
	cgroupPath, err := c.CGroupPath()("")
	if err != nil {
		return nil, errors.Wrapf(err, "error getting cgroup path for container %s", c.ID())
	}

	taskFile := filepath.Join("/sys/fs/cgroup/pids", cgroupPath, "tasks")

	logrus.Debug("reading pids from ", taskFile)

	content, err := ioutil.ReadFile(taskFile)
	if err != nil {
		return []string{}, errors.Wrapf(err, "unable to read pids from %s", taskFile)
	}
	return strings.Fields(string(content)), nil
}

// GetContainerPidInformation calls ps with the appropriate options and returns
// the results as a string and the container's PIDs as a []string
func (c *Container) GetContainerPidInformation(args []string) ([]string, error) {
	if !c.batched {
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

	filteredOutput, err := filterPids(results, pids)
	if err != nil {
		return []string{}, err
	}
	return filteredOutput, nil
}

func filterPids(psOutput string, pids []string) ([]string, error) {
	var output []string
	results := strings.Split(psOutput, "\n")
	// The headers are in the first line of the results
	headers := fieldsASCII(results[0])
	// We need to make sure PID in headers, so that we can filter
	// Pids that don't belong.

	// append the headers back in
	output = append(output, results[0])

	pidIndex := -1
	for i, header := range headers {
		if header == "PID" {
			pidIndex = i
		}
	}
	if pidIndex == -1 {
		return []string{}, errors.Errorf("unable to find PID field in ps output. try a different set of ps arguments")
	}
	for _, l := range results[1:] {
		if l == "" {
			continue
		}
		cols := fieldsASCII(l)
		pid := cols[pidIndex]
		if util.StringInSlice(pid, pids) {
			output = append(output, l)
		}
	}
	return output, nil
}

// Detects ascii whitespaces
func fieldsASCII(s string) []string {
	fn := func(r rune) bool {
		switch r {
		case '\t', '\n', '\f', '\r', ' ':
			return true
		}
		return false
	}
	return strings.FieldsFunc(s, fn)
}
