package cgroups

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type cpusetHandler struct {
}

func cpusetCopyFileFromParent(dir, file string, cgroupv2 bool) ([]byte, error) {
	if dir == cgroupRoot {
		return nil, fmt.Errorf("could not find parent to initialize cpuset %s", file)
	}
	path := filepath.Join(dir, file)
	parentPath := path
	if cgroupv2 {
		parentPath = fmt.Sprintf("%s.effective", parentPath)
	}
	data, err := ioutil.ReadFile(parentPath)
	if err != nil {
		return nil, errors.Wrapf(err, "open %s", path)
	}
	if len(strings.Trim(string(data), "\n")) != 0 {
		return data, nil
	}
	data, err = cpusetCopyFileFromParent(filepath.Dir(dir), file, cgroupv2)
	if err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return nil, errors.Wrapf(err, "write %s", path)
	}
	return data, nil
}

func cpusetCopyFromParent(path string, cgroupv2 bool) error {
	for _, file := range []string{"cpuset.cpus", "cpuset.mems"} {
		if _, err := cpusetCopyFileFromParent(path, file, cgroupv2); err != nil {
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
		path := filepath.Join(cgroupRoot, ctr.path)
		return true, cpusetCopyFromParent(path, true)
	}

	created, err := ctr.createCgroupDirectory(CPUset)
	if !created || err != nil {
		return created, err
	}
	return true, cpusetCopyFromParent(ctr.getCgroupv1Path(CPUset), false)
}

// Destroy the cgroup
func (c *cpusetHandler) Destroy(ctr *CgroupControl) error {
	return rmDirRecursively(ctr.getCgroupv1Path(CPUset))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *cpusetHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	return nil
}
