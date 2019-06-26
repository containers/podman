package cgroups

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type cpusetHandler struct {
}

func cpusetCopyFileFromParent(dir, file string) ([]byte, error) {
	if dir == cgroupRoot {
		return nil, fmt.Errorf("could not find parent to initialize cpuset %s", file)
	}
	path := filepath.Join(dir, file)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "open %s", path)
	}
	if len(strings.Trim(string(data), "\n")) != 0 {
		return data, nil
	}
	data, err = cpusetCopyFileFromParent(filepath.Dir(dir), file)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return nil, errors.Wrapf(err, "write %s", path)
	}
	return data, nil
}

func cpusetCopyFromParent(path string) error {
	for _, file := range []string{"cpuset.cpus", "cpuset.mems"} {
		if _, err := cpusetCopyFileFromParent(path, file); err != nil {
			return err
		}
	}
	return nil
}

func getCpusetHandler() *cpusetHandler {
	return &cpusetHandler{}
}

// Apply set the specified constraints
func (c *cpusetHandler) Apply(ctr *CgroupControl, res *spec.LinuxResources) error {
	if res.CPU == nil {
		return nil
	}
	return fmt.Errorf("cpuset apply not implemented yet")
}

// Create the cgroup
func (c *cpusetHandler) Create(ctr *CgroupControl) (bool, error) {
	if ctr.cgroup2 {
		return false, fmt.Errorf("cpuset create not implemented for cgroup v2")
	}

	created, err := ctr.createCgroupDirectory(CPUset)
	if !created || err != nil {
		return created, err
	}
	return true, cpusetCopyFromParent(ctr.getCgroupv1Path(CPUset))
}

// Destroy the cgroup
func (c *cpusetHandler) Destroy(ctr *CgroupControl) error {
	return os.Remove(ctr.getCgroupv1Path(CPUset))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *cpusetHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	return nil
}
