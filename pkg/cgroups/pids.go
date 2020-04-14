package cgroups

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

type pidHandler struct {
}

func getPidsHandler() *pidHandler {
	return &pidHandler{}
}

// Apply set the specified constraints
func (c *pidHandler) Apply(ctr *CgroupControl, res *spec.LinuxResources) error {
	if res.Pids == nil {
		return nil
	}
	var PIDRoot string

	if ctr.cgroup2 {
		PIDRoot = filepath.Join(cgroupRoot, ctr.path)
	} else {
		PIDRoot = ctr.getCgroupv1Path(Pids)
	}

	p := filepath.Join(PIDRoot, "pids.max")
	return ioutil.WriteFile(p, []byte(fmt.Sprintf("%d\n", res.Pids.Limit)), 0644)
}

// Create the cgroup
func (c *pidHandler) Create(ctr *CgroupControl) (bool, error) {
	return ctr.createCgroupDirectory(Pids)
}

// Destroy the cgroup
func (c *pidHandler) Destroy(ctr *CgroupControl) error {
	return rmDirRecursively(ctr.getCgroupv1Path(Pids))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *pidHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	if ctr.path == "" {
		// nothing we can do to retrieve the pids.current path
		return nil
	}

	var PIDRoot string
	if ctr.cgroup2 {
		PIDRoot = filepath.Join(cgroupRoot, ctr.path)
	} else {
		PIDRoot = ctr.getCgroupv1Path(Pids)
	}

	current, err := readFileAsUint64(filepath.Join(PIDRoot, "pids.current"))
	if err != nil {
		return err
	}

	m.Pids = PidsMetrics{Current: current}
	return nil
}
