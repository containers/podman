package cgroups

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var TestMode bool

func cleanString(s string) string {
	return strings.Trim(s, "\n")
}

func readAcct(ctr *CgroupControl, name string) (uint64, error) {
	p := filepath.Join(ctr.getCgroupv1Path(CPUAcct), name)
	return readFileAsUint64(p)
}

func readAcctList(ctr *CgroupControl, name string) ([]uint64, error) {
	p := filepath.Join(ctr.getCgroupv1Path(CPUAcct), name)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	r := []uint64{}
	for _, s := range strings.Split(string(data), " ") {
		s = cleanString(s)
		if s == "" {
			break
		}
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", s, err)
		}
		r = append(r, v)
	}
	return r, nil
}

// GetSystemCPUUsage returns the system usage for all the cgroups
func GetSystemCPUUsage() (uint64, error) {
	cgroupv2, err := IsCgroup2UnifiedMode()
	if err != nil {
		return 0, err
	}
	if !cgroupv2 {
		p := filepath.Join(cgroupRoot, CPUAcct, "cpuacct.usage")
		return readFileAsUint64(p)
	}

	files, err := os.ReadDir(cgroupRoot)
	if err != nil {
		return 0, err
	}
	var total uint64
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		p := filepath.Join(cgroupRoot, file.Name(), "cpu.stat")

		values, err := readCgroup2MapPath(p)
		if err != nil {
			return 0, err
		}

		if val, found := values["usage_usec"]; found {
			v, err := strconv.ParseUint(cleanString(val[0]), 10, 64)
			if err != nil {
				return 0, err
			}
			total += v * 1000
		}
	}
	return total, nil
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
	data, err := os.ReadFile(parentPath)
	if err != nil {
		// if the file doesn't exist, it is likely that the cpuset controller
		// is not enabled in the kernel.
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if strings.Trim(string(data), "\n") != "" {
		return data, nil
	}
	data, err = cpusetCopyFileFromParent(filepath.Dir(dir), file, cgroupv2)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
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

// createCgroupv2Path creates the cgroupv2 path and enables all the available controllers
func createCgroupv2Path(path string) (deferredError error) {
	if !strings.HasPrefix(path, cgroupRoot+"/") {
		return fmt.Errorf("invalid cgroup path %s", path)
	}
	content, err := os.ReadFile(cgroupRoot + "/cgroup.controllers")
	if err != nil {
		return err
	}
	ctrs := bytes.Fields(content)
	res := append([]byte("+"), bytes.Join(ctrs, []byte(" +"))...)

	current := "/sys/fs"
	elements := strings.Split(path, "/")
	for i, e := range elements[3:] {
		current = filepath.Join(current, e)
		if i > 0 {
			if err := os.Mkdir(current, 0o755); err != nil {
				if !os.IsExist(err) {
					return err
				}
			} else {
				// If the directory was created, be sure it is not left around on errors.
				defer func() {
					if deferredError != nil {
						os.Remove(current)
					}
				}()
			}
		}
		// We enable the controllers for all the path components except the last one.  It is not allowed to add
		// PIDs if there are already enabled controllers.
		if i < len(elements[3:])-1 {
			if err := os.WriteFile(filepath.Join(current, "cgroup.subtree_control"), res, 0o755); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *CgroupControl) createCgroupDirectory(controller string) (bool, error) {
	cPath := c.getCgroupv1Path(controller)
	_, err := os.Stat(cPath)
	if err == nil {
		return false, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	if err := os.MkdirAll(cPath, 0o755); err != nil {
		return false, fmt.Errorf("creating cgroup for %s: %w", controller, err)
	}
	return true, nil
}
