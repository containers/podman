//go:build linux || darwin
// +build linux darwin

package utils

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/pkg/rootless"
	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/sirupsen/logrus"
)

// RunUnderSystemdScope adds the specified pid to a systemd scope
func RunUnderSystemdScope(pid int, slice string, unitName string) error {
	var properties []systemdDbus.Property
	var conn *systemdDbus.Conn
	var err error

	if rootless.IsRootless() {
		conn, err = cgroups.GetUserConnection(rootless.GetRootlessUID())
		if err != nil {
			return err
		}
	} else {
		conn, err = systemdDbus.NewWithContext(context.Background())
		if err != nil {
			return err
		}
	}
	defer conn.Close()
	properties = append(properties, systemdDbus.PropSlice(slice))
	properties = append(properties, newProp("PIDs", []uint32{uint32(pid)}))
	properties = append(properties, newProp("Delegate", true))
	properties = append(properties, newProp("DefaultDependencies", false))
	ch := make(chan string)
	_, err = conn.StartTransientUnitContext(context.Background(), unitName, "replace", properties, ch)
	if err != nil {
		// On errors check if the cgroup already exists, if it does move the process there
		if props, err := conn.GetUnitTypePropertiesContext(context.Background(), unitName, "Scope"); err == nil {
			if cgroup, ok := props["ControlGroup"].(string); ok && cgroup != "" {
				if err := moveUnderCgroup(cgroup, "", []uint32{uint32(pid)}); err == nil {
					return nil
				}
				// On errors return the original error message we got from StartTransientUnit.
			}
		}
		return err
	}

	// Block until job is started
	<-ch

	return nil
}

func getCgroupProcess(procFile string, allowRoot bool) (string, error) {
	f, err := os.Open(procFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	cgroup := ""
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			return "", fmt.Errorf("cannot parse cgroup line %q", line)
		}
		if strings.HasPrefix(line, "0::") {
			cgroup = line[3:]
			break
		}
		if len(parts[2]) > len(cgroup) {
			cgroup = parts[2]
		}
	}
	if len(cgroup) == 0 || (!allowRoot && cgroup == "/") {
		return "", fmt.Errorf("could not find cgroup mount in %q", procFile)
	}
	return cgroup, nil
}

// GetOwnCgroup returns the cgroup for the current process.
func GetOwnCgroup() (string, error) {
	return getCgroupProcess("/proc/self/cgroup", true)
}

func GetOwnCgroupDisallowRoot() (string, error) {
	return getCgroupProcess("/proc/self/cgroup", false)
}

// GetCgroupProcess returns the cgroup for the specified process process.
func GetCgroupProcess(pid int) (string, error) {
	return getCgroupProcess(fmt.Sprintf("/proc/%d/cgroup", pid), true)
}

// MoveUnderCgroupSubtree moves the PID under a cgroup subtree.
func MoveUnderCgroupSubtree(subtree string) error {
	return moveUnderCgroup("", subtree, nil)
}

// moveUnderCgroup moves a group of processes to a new cgroup.
// If cgroup is the empty string, then the current calling process cgroup is used.
// If processes is empty, then the processes from the current cgroup are moved.
func moveUnderCgroup(cgroup, subtree string, processes []uint32) error {
	procFile := "/proc/self/cgroup"
	f, err := os.Open(procFile)
	if err != nil {
		return err
	}
	defer f.Close()

	unifiedMode, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			return fmt.Errorf("cannot parse cgroup line %q", line)
		}

		// root cgroup, skip it
		if parts[2] == "/" && !(unifiedMode && parts[1] == "") {
			continue
		}

		cgroupRoot := "/sys/fs/cgroup"
		// Special case the unified mount on hybrid cgroup and named hierarchies.
		// This works on Fedora 31, but we should really parse the mounts to see
		// where the cgroup hierarchy is mounted.
		if parts[1] == "" && !unifiedMode {
			// If it is not using unified mode, the cgroup v2 hierarchy is
			// usually mounted under /sys/fs/cgroup/unified
			cgroupRoot = filepath.Join(cgroupRoot, "unified")

			// Ignore the unified mount if it doesn't exist
			if _, err := os.Stat(cgroupRoot); err != nil && os.IsNotExist(err) {
				continue
			}
		} else if parts[1] != "" {
			// Assume the controller is mounted at /sys/fs/cgroup/$CONTROLLER.
			controller := strings.TrimPrefix(parts[1], "name=")
			cgroupRoot = filepath.Join(cgroupRoot, controller)
		}

		parentCgroup := cgroup
		if parentCgroup == "" {
			parentCgroup = parts[2]
		}
		newCgroup := filepath.Join(cgroupRoot, parentCgroup, subtree)
		if err := os.MkdirAll(newCgroup, 0755); err != nil && !os.IsExist(err) {
			return err
		}

		f, err := os.OpenFile(filepath.Join(newCgroup, "cgroup.procs"), os.O_RDWR, 0755)
		if err != nil {
			return err
		}
		defer f.Close()

		if len(processes) > 0 {
			for _, pid := range processes {
				if _, err := f.Write([]byte(fmt.Sprintf("%d\n", pid))); err != nil {
					logrus.Debugf("Cannot move process %d to cgroup %q: %v", pid, newCgroup, err)
				}
			}
		} else {
			processesData, err := ioutil.ReadFile(filepath.Join(cgroupRoot, parts[2], "cgroup.procs"))
			if err != nil {
				return err
			}
			for _, pid := range bytes.Split(processesData, []byte("\n")) {
				if len(pid) == 0 {
					continue
				}
				if _, err := f.Write(pid); err != nil {
					logrus.Debugf("Cannot move process %s to cgroup %q: %v", string(pid), newCgroup, err)
				}
			}
		}
	}
	return nil
}

func newProp(name string, units interface{}) systemdDbus.Property {
	return systemdDbus.Property{
		Name:  name,
		Value: dbus.MakeVariant(units),
	}
}
