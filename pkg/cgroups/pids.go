package cgroups

import (
	"fmt"
	"io/ioutil"
	"os"
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
	if ctr.cgroup2 {
		return false, fmt.Errorf("pid create not implemented for cgroup v2")
	}
	return ctr.createCgroupDirectory(Pids)
}

// Destroy the cgroup
func (c *pidHandler) Destroy(ctr *CgroupControl) error {
	return os.Remove(ctr.getCgroupv1Path(Pids))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *pidHandler) Stat(ctr *CgroupControl, m *Metrics) error {
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
