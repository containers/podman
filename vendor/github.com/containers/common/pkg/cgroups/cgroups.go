//go:build !linux
// +build !linux

package cgroups

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/storage/pkg/unshare"
	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

var (
	// ErrCgroupDeleted means the cgroup was deleted
	ErrCgroupDeleted = errors.New("cgroup deleted")
	// ErrCgroupV1Rootless means the cgroup v1 were attempted to be used in rootless environment
	ErrCgroupV1Rootless = errors.New("no support for CGroups V1 in rootless environments")
	ErrStatCgroup       = errors.New("no cgroup available for gathering user statistics")
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

// CPUUsage keeps stats for the CPU usage (unit: nanoseconds)
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
	cgroupRoot = "/sys/fs/cgroup"
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
		controllers := []controller{}
		controllersFile := cgroupRoot + "/cgroup.controllers"
		// rootless cgroupv2: check available controllers for current user, systemd or servicescope will inherit
		if unshare.IsRootless() {
			userSlice, err := getCgroupPathForCurrentProcess()
			if err != nil {
				return controllers, err
			}
			// userSlice already contains '/' so not adding here
			basePath := cgroupRoot + userSlice
			controllersFile = fmt.Sprintf("%s/cgroup.controllers", basePath)
		}
		controllersFileBytes, err := ioutil.ReadFile(controllersFile)
		if err != nil {
			return nil, fmt.Errorf("failed while reading controllers for cgroup v2 from %q: %w", controllersFile, err)
		}
		for _, controllerName := range strings.Fields(string(controllersFileBytes)) {
			c := controller{
				name:    controllerName,
				symlink: false,
			}
			controllers = append(controllers, c)
		}
		return controllers, nil
	}

	subsystems, _ := cgroupV1GetAllSubsystems()
	controllers := []controller{}
	// cgroupv1 and rootless: No subsystem is available: delegation is unsafe.
	if unshare.IsRootless() {
		return controllers, nil
	}

	for _, name := range subsystems {
		if _, found := exclude[name]; found {
			continue
		}
		fileInfo, err := os.Stat(cgroupRoot + "/" + name)
		if err != nil {
			continue
		}
		c := controller{
			name:    name,
			symlink: !fileInfo.IsDir(),
		}
		controllers = append(controllers, c)
	}

	return controllers, nil
}

// GetAvailableControllers get string:bool map of all the available controllers
func GetAvailableControllers(exclude map[string]controllerHandler, cgroup2 bool) ([]string, error) {
	availableControllers, err := getAvailableControllers(exclude, cgroup2)
	if err != nil {
		return nil, err
	}
	controllerList := []string{}
	for _, controller := range availableControllers {
		controllerList = append(controllerList, controller.name)
	}

	return controllerList, nil
}

func cgroupV1GetAllSubsystems() ([]string, error) {
	f, err := os.Open("/proc/cgroups")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	subsystems := []string{}

	s := bufio.NewScanner(f)
	for s.Scan() {
		text := s.Text()
		if text[0] != '#' {
			parts := strings.Fields(text)
			if len(parts) >= 4 && parts[3] != "0" {
				subsystems = append(subsystems, parts[0])
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return subsystems, nil
}

func getCgroupPathForCurrentProcess() (string, error) {
	path := fmt.Sprintf("/proc/%d/cgroup", os.Getpid())
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	cgroupPath := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		text := s.Text()
		procEntries := strings.SplitN(text, "::", 2)
		// set process cgroupPath only if entry is valid
		if len(procEntries) > 1 {
			cgroupPath = procEntries[1]
		}
	}
	if err := s.Err(); err != nil {
		return cgroupPath, err
	}
	return cgroupPath, nil
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
	if c.cgroup2 {
		if err := createCgroupv2Path(filepath.Join(cgroupRoot, c.path)); err != nil {
			return fmt.Errorf("error creating cgroup path %s: %w", c.path, err)
		}
	}
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
			if err := os.MkdirAll(path, 0o755); err != nil {
				return fmt.Errorf("error creating cgroup path for %s: %w", ctr.name, err)
			}
		}
	}

	return nil
}

func readFileAsUint64(path string) (uint64, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v := cleanString(string(data))
	if v == "max" {
		return math.MaxUint64, nil
	}
	ret, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return ret, fmt.Errorf("parse %s from %s: %w", v, path, err)
	}
	return ret, nil
}

func readFileByKeyAsUint64(path, key string) (uint64, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.SplitN(line, " ", 2)
		if fields[0] == key {
			v := cleanString(string(fields[1]))
			if v == "max" {
				return math.MaxUint64, nil
			}
			ret, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return ret, fmt.Errorf("parse %s from %s: %w", v, path, err)
			}
			return ret, nil
		}
	}

	return 0, fmt.Errorf("no key named %s from %s", key, path)
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
		controllers, err := getAvailableControllers(handlers, false)
		if err != nil {
			return nil, err
		}
		control.additionalControllers = controllers
	}
	if !cgroup2 {
		oneExists := false
		// check that the cgroup exists at least under one controller
		for name := range handlers {
			p := control.getCgroupv1Path(name)
			if _, err := os.Stat(p); err == nil {
				oneExists = true
				break
			}
		}

		// if there is no controller at all, raise an error
		if !oneExists {
			if unshare.IsRootless() {
				return nil, ErrCgroupV1Rootless
			}
			// compatible with the error code
			// used by containerd/cgroups
			return nil, ErrCgroupDeleted
		}
	}
	return control, nil
}

// CreateSystemdUnit creates the systemd cgroup
func (c *CgroupControl) CreateSystemdUnit(path string) error {
	if !c.systemd {
		return fmt.Errorf("the cgroup controller is not using systemd")
	}

	conn, err := systemdDbus.NewWithContext(context.TODO())
	if err != nil {
		return err
	}
	defer conn.Close()

	return systemdCreate(path, conn)
}

// GetUserConnection returns an user connection to D-BUS
func GetUserConnection(uid int) (*systemdDbus.Conn, error) {
	return systemdDbus.NewConnection(func() (*dbus.Conn, error) {
		return dbusAuthConnection(uid, dbus.SessionBusPrivateNoAutoStartup)
	})
}

// CreateSystemdUserUnit creates the systemd cgroup for the specified user
func (c *CgroupControl) CreateSystemdUserUnit(path string, uid int) error {
	if !c.systemd {
		return fmt.Errorf("the cgroup controller is not using systemd")
	}

	conn, err := GetUserConnection(uid)
	if err != nil {
		return err
	}
	defer conn.Close()

	return systemdCreate(path, conn)
}

func dbusAuthConnection(uid int, createBus func(opts ...dbus.ConnOption) (*dbus.Conn, error)) (*dbus.Conn, error) {
	conn, err := createBus()
	if err != nil {
		return nil, err
	}

	methods := []dbus.Auth{dbus.AuthExternal(strconv.Itoa(uid))}

	err = conn.Auth(methods)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Hello(); err != nil {
		return nil, err
	}

	return conn, nil
}

// Delete cleans a cgroup
func (c *CgroupControl) Delete() error {
	return c.DeleteByPath(c.path)
}

// DeleteByPathConn deletes the specified cgroup path using the specified
// dbus connection if needed.
func (c *CgroupControl) DeleteByPathConn(path string, conn *systemdDbus.Conn) error {
	if c.systemd {
		return systemdDestroyConn(path, conn)
	}
	if c.cgroup2 {
		return rmDirRecursively(filepath.Join(cgroupRoot, c.path))
	}
	var lastError error
	for _, h := range handlers {
		if err := h.Destroy(c); err != nil {
			lastError = err
		}
	}

	for _, ctr := range c.additionalControllers {
		if ctr.symlink {
			continue
		}
		p := c.getCgroupv1Path(ctr.name)
		if err := rmDirRecursively(p); err != nil {
			lastError = fmt.Errorf("remove %s: %w", p, err)
		}
	}
	return lastError
}

// DeleteByPath deletes the specified cgroup path
func (c *CgroupControl) DeleteByPath(path string) error {
	if c.systemd {
		conn, err := systemdDbus.NewWithContext(context.TODO())
		if err != nil {
			return err
		}
		defer conn.Close()
		return c.DeleteByPathConn(path, conn)
	}
	return c.DeleteByPathConn(path, nil)
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
		p := filepath.Join(cgroupRoot, c.path, "cgroup.procs")
		if err := ioutil.WriteFile(p, pidString, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", p, err)
		}
		return nil
	}

	names := make([]string, 0, len(handlers))
	for n := range handlers {
		names = append(names, n)
	}

	for _, c := range c.additionalControllers {
		if !c.symlink {
			names = append(names, c.name)
		}
	}

	for _, n := range names {
		// If we aren't using cgroup2, we won't write correctly to unified hierarchy
		if !c.cgroup2 && n == "unified" {
			continue
		}
		p := filepath.Join(c.getCgroupv1Path(n), "tasks")
		if err := ioutil.WriteFile(p, pidString, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", p, err)
		}
	}
	return nil
}

// Stat returns usage statistics for the cgroup
func (c *CgroupControl) Stat() (*Metrics, error) {
	m := Metrics{}
	found := false
	for _, h := range handlers {
		if err := h.Stat(c, &m); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			logrus.Warningf("Failed to retrieve cgroup stats: %v", err)
			continue
		}
		found = true
	}
	if !found {
		return nil, ErrStatCgroup
	}
	return &m, nil
}

func readCgroup2MapPath(path string) (map[string][]string, error) {
	ret := map[string][]string{}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ret, nil
		}
		return nil, fmt.Errorf("open file %s: %w", path, err)
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
		return nil, fmt.Errorf("parsing file %s: %w", path, err)
	}
	return ret, nil
}

func readCgroup2MapFile(ctr *CgroupControl, name string) (map[string][]string, error) {
	p := filepath.Join(cgroupRoot, ctr.path, name)

	return readCgroup2MapPath(p)
}
