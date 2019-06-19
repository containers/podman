package cgroups

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrCgroupDeleted means the cgroup was deleted
	ErrCgroupDeleted = errors.New("cgroups: cgroup deleted")
)

// CgroupControl controls a cgroup hierarchy
type CgroupControl struct {
	cgroup2 bool
	path    string
	systemd bool
	// List of additional cgroup subsystems joined that
	// do not have a custom handler.
	additionalControllers []controller
}

// CPUUsage keeps stats for the CPU usage
type CPUUsage struct {
	Kernel uint64
	Total  uint64
	PerCPU []uint64
}

// MemoryUsage keeps stats for the memory usage
type MemoryUsage struct {
	Usage uint64
	Limit uint64
}

// CPUMetrics keeps stats for the CPU usage
type CPUMetrics struct {
	Usage CPUUsage
}

// BlkIOEntry describes an entry in the blkio stats
type BlkIOEntry struct {
	Op    string
	Major uint64
	Minor uint64
	Value uint64
}

// BlkioMetrics keeps usage stats for the blkio cgroup controller
type BlkioMetrics struct {
	IoServiceBytesRecursive []BlkIOEntry
}

// MemoryMetrics keeps usage stats for the memory cgroup controller
type MemoryMetrics struct {
	Usage MemoryUsage
}

// PidsMetrics keeps usage stats for the pids cgroup controller
type PidsMetrics struct {
	Current uint64
}

// Metrics keeps usage stats for the cgroup controllers
type Metrics struct {
	CPU    CPUMetrics
	Blkio  BlkioMetrics
	Memory MemoryMetrics
	Pids   PidsMetrics
}

type controller struct {
	name    string
	symlink bool
}

type controllerHandler interface {
	Create(*CgroupControl) (bool, error)
	Apply(*CgroupControl, *spec.LinuxResources) error
	Destroy(*CgroupControl) error
	Stat(*CgroupControl, *Metrics) error
}

const (
	cgroupRoot         = "/sys/fs/cgroup"
	_cgroup2SuperMagic = 0x63677270
	// CPU is the cpu controller
	CPU = "cpu"
	// CPUAcct is the cpuacct controller
	CPUAcct = "cpuacct"
	// CPUset is the cpuset controller
	CPUset = "cpuset"
	// Memory is the memory controller
	Memory = "memory"
	// Pids is the pids controller
	Pids = "pids"
	// Blkio is the blkio controller
	Blkio = "blkio"
)

var handlers map[string]controllerHandler

func init() {
	handlers = make(map[string]controllerHandler)
	handlers[CPU] = getCPUHandler()
	handlers[CPUset] = getCpusetHandler()
	handlers[Memory] = getMemoryHandler()
	handlers[Pids] = getPidsHandler()
	handlers[Blkio] = getBlkioHandler()
}

// getAvailableControllers get the available controllers
func getAvailableControllers(exclude map[string]controllerHandler, cgroup2 bool) ([]controller, error) {
	if cgroup2 {
		return nil, fmt.Errorf("getAvailableControllers not implemented yet for cgroup v2")
	}

	infos, err := ioutil.ReadDir(cgroupRoot)
	if err != nil {
		return nil, errors.Wrapf(err, "read directory %s", cgroupRoot)
	}
	var controllers []controller
	for _, i := range infos {
		name := i.Name()
		if _, found := exclude[name]; found {
			continue
		}
		c := controller{
			name:    name,
			symlink: !i.IsDir(),
		}
		controllers = append(controllers, c)
	}
	return controllers, nil
}

// getCgroupv1Path is a helper function to get the cgroup v1 path
func (c *CgroupControl) getCgroupv1Path(name string) string {
	return filepath.Join(cgroupRoot, name, c.path)
}

// initialize initializes the specified hierarchy
func (c *CgroupControl) initialize() (err error) {
	createdSoFar := map[string]controllerHandler{}
	defer func() {
		if err != nil {
			for name, ctr := range createdSoFar {
				if err := ctr.Destroy(c); err != nil {
					logrus.Warningf("error cleaning up controller %s for %s", name, c.path)
				}
			}
		}
	}()
	for name, handler := range handlers {
		created, err := handler.Create(c)
		if err != nil {
			return err
		}
		if created {
			createdSoFar[name] = handler
		}
	}

	if !c.cgroup2 {
		// We won't need to do this for cgroup v2
		for _, ctr := range c.additionalControllers {
			if ctr.symlink {
				continue
			}
			path := c.getCgroupv1Path(ctr.name)
			if err := os.MkdirAll(path, 0755); err != nil {
				return errors.Wrapf(err, "error creating cgroup path %s for %s", path, ctr.name)
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

	if !os.IsNotExist(err) {
		return false, err
	}

	if err := os.MkdirAll(cPath, 0755); err != nil {
		return false, errors.Wrapf(err, "error creating cgroup for %s", controller)
	}
	return true, nil
}

func readFileAsUint64(path string) (uint64, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, errors.Wrapf(err, "open %s", path)
	}
	v := cleanString(string(data))
	if v == "max" {
		return math.MaxUint64, nil
	}
	ret, err := strconv.ParseUint(v, 10, 0)
	if err != nil {
		return ret, errors.Wrapf(err, "parse %s from %s", v, path)
	}
	return ret, nil
}

func (c *CgroupControl) writePidToTasks(pid int, name string) error {
	path := filepath.Join(c.getCgroupv1Path(name), "tasks")
	payload := []byte(fmt.Sprintf("%d", pid))
	return ioutil.WriteFile(path, payload, 0644)
}

// New creates a new cgroup control
func New(path string, resources *spec.LinuxResources) (*CgroupControl, error) {
	cgroup2, err := IsCgroup2UnifiedMode()
	if err != nil {
		return nil, err
	}
	control := &CgroupControl{
		cgroup2: cgroup2,
		path:    path,
	}

	if !cgroup2 {
		controllers, err := getAvailableControllers(handlers, false)
		if err != nil {
			return nil, err
		}
		control.additionalControllers = controllers
	}

	if err := control.initialize(); err != nil {
		return nil, err
	}

	return control, nil
}

// NewSystemd creates a new cgroup control
func NewSystemd(path string) (*CgroupControl, error) {
	cgroup2, err := IsCgroup2UnifiedMode()
	if err != nil {
		return nil, err
	}
	control := &CgroupControl{
		cgroup2: cgroup2,
		path:    path,
		systemd: true,
	}
	return control, nil
}

// Load loads an existing cgroup control
func Load(path string) (*CgroupControl, error) {
	cgroup2, err := IsCgroup2UnifiedMode()
	if err != nil {
		return nil, err
	}
	control := &CgroupControl{
		cgroup2: cgroup2,
		path:    path,
		systemd: false,
	}
	if !cgroup2 {
		for name := range handlers {
			p := control.getCgroupv1Path(name)
			if _, err := os.Stat(p); err != nil {
				if os.IsNotExist(err) {
					// compatible with the error code
					// used by containerd/cgroups
					return nil, ErrCgroupDeleted
				}
			}
		}
	}
	return control, nil
}

// CreateSystemdUnit creates the systemd cgroup
func (c *CgroupControl) CreateSystemdUnit(path string) error {
	if !c.systemd {
		return fmt.Errorf("the cgroup controller is not using systemd")
	}
	return systemdCreate(path)
}

// Delete cleans a cgroup
func (c *CgroupControl) Delete() error {
	return c.DeleteByPath(c.path)
}

// DeleteByPath deletes the specified cgroup path
func (c *CgroupControl) DeleteByPath(path string) error {
	if c.systemd {
		return systemdDestroy(path)
	}
	var lastError error
	for _, h := range handlers {
		if err := h.Destroy(c); err != nil {
			lastError = err
		}
	}

	for _, ctr := range c.additionalControllers {
		p := c.getCgroupv1Path(ctr.name)
		if err := os.Remove(p); err != nil {
			lastError = errors.Wrapf(err, "remove %s", p)
		}
	}
	return lastError
}

// Update updates the cgroups
func (c *CgroupControl) Update(resources *spec.LinuxResources) error {
	for _, h := range handlers {
		if err := h.Apply(c, resources); err != nil {
			return err
		}
	}
	return nil
}

// AddPid moves the specified pid to the cgroup
func (c *CgroupControl) AddPid(pid int) error {
	pidString := []byte(fmt.Sprintf("%d\n", pid))

	if c.cgroup2 {
		p := filepath.Join(cgroupRoot, c.path, "tasks")
		if err := ioutil.WriteFile(p, pidString, 0644); err != nil {
			return errors.Wrapf(err, "write %s", p)
		}
		return nil
	}

	var names []string
	for n := range handlers {
		names = append(names, n)
	}

	for _, c := range c.additionalControllers {
		if !c.symlink {
			names = append(names, c.name)
		}
	}

	for _, n := range names {
		p := filepath.Join(c.getCgroupv1Path(n), "tasks")
		if err := ioutil.WriteFile(p, pidString, 0644); err != nil {
			return errors.Wrapf(err, "write %s", p)
		}
	}
	return nil
}

// Stat returns usage statistics for the cgroup
func (c *CgroupControl) Stat() (*Metrics, error) {
	m := Metrics{}
	for _, h := range handlers {
		if err := h.Stat(c, &m); err != nil {
			return nil, err
		}
	}
	return &m, nil
}

func readCgroup2MapFile(ctr *CgroupControl, name string) (map[string][]string, error) {
	ret := map[string][]string{}
	p := filepath.Join(cgroupRoot, ctr.path, name)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ret, nil
		}
		return nil, errors.Wrapf(err, "open file %s", p)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ret[parts[0]] = parts[1:]
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "parsing file %s", p)
	}
	return ret, nil
}
