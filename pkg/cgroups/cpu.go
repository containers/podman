package cgroups

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type cpuHandler struct {
}

func getCPUHandler() *cpuHandler {
	return &cpuHandler{}
}

func cleanString(s string) string {
	return strings.Trim(s, "\n")
}

func readAcct(ctr *CgroupControl, name string) (uint64, error) {
	p := filepath.Join(ctr.getCgroupv1Path(CPUAcct), name)
	return readFileAsUint64(p)
}

func readAcctList(ctr *CgroupControl, name string) ([]uint64, error) {
	var r []uint64

	p := filepath.Join(ctr.getCgroupv1Path(CPUAcct), name)
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", p)
	}
	for _, s := range strings.Split(string(data), " ") {
		s = cleanString(s)
		if s == "" {
			break
		}
		v, err := strconv.ParseUint(s, 10, 0)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s", s)
		}
		r = append(r, v)
	}
	return r, nil
}

// Apply set the specified constraints
func (c *cpuHandler) Apply(ctr *CgroupControl, res *spec.LinuxResources) error {
	if res.CPU == nil {
		return nil
	}
	return fmt.Errorf("cpu apply not implemented yet")
}

// Create the cgroup
func (c *cpuHandler) Create(ctr *CgroupControl) (bool, error) {
	if ctr.cgroup2 {
		return false, fmt.Errorf("cpu create not implemented for cgroup v2")
	}
	return ctr.createCgroupDirectory(CPU)
}

// Destroy the cgroup
func (c *cpuHandler) Destroy(ctr *CgroupControl) error {
	return os.Remove(ctr.getCgroupv1Path(CPU))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *cpuHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	var err error
	usage := CPUUsage{}
	if ctr.cgroup2 {
		values, err := readCgroup2MapFile(ctr, "cpu.stat")
		if err != nil {
			return err
		}
		if val, found := values["usage_usec"]; found {
			usage.Kernel, err = strconv.ParseUint(cleanString(val[0]), 10, 0)
			if err != nil {
				return err
			}
		}
		if val, found := values["system_usec"]; found {
			usage.Total, err = strconv.ParseUint(cleanString(val[0]), 10, 0)
			if err != nil {
				return err
			}
		}
		// FIXME: How to read usage.PerCPU?
	} else {
		usage.Total, err = readAcct(ctr, "cpuacct.usage")
		if err != nil {
			return err
		}
		usage.Kernel, err = readAcct(ctr, "cpuacct.usage_sys")
		if err != nil {
			return err
		}
		usage.PerCPU, err = readAcctList(ctr, "cpuacct.usage_percpu")
		if err != nil {
			return err
		}
	}
	m.CPU = CPUMetrics{Usage: usage}
	return nil
}
