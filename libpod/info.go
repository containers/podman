package libpod

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/seccomp"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/linkmode"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/system"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Info returns the store and host information
func (r *Runtime) info() (*define.Info, error) {
	info := define.Info{}
	versionInfo, err := define.GetVersion()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting version info")
	}
	info.Version = versionInfo
	// get host information
	hostInfo, err := r.hostInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting host info")
	}
	info.Host = hostInfo

	// get store information
	storeInfo, err := r.storeInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting store info")
	}
	info.Store = storeInfo
	registries := make(map[string]interface{})

	sys := r.SystemContext()
	data, err := sysregistriesv2.GetRegistries(sys)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting registries")
	}
	for _, reg := range data {
		registries[reg.Prefix] = reg
	}
	regs, err := sysregistriesv2.UnqualifiedSearchRegistries(sys)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting registries")
	}
	if len(regs) > 0 {
		registries["search"] = regs
	}
	volumePlugins := make([]string, 0, len(r.config.Engine.VolumePlugins)+1)
	// the local driver always exists
	volumePlugins = append(volumePlugins, "local")
	for plugin := range r.config.Engine.VolumePlugins {
		volumePlugins = append(volumePlugins, plugin)
	}
	info.Plugins.Volume = volumePlugins
	info.Plugins.Network = r.network.Drivers()
	info.Plugins.Log = logDrivers

	info.Registries = registries
	return &info, nil
}

// top-level "host" info
func (r *Runtime) hostInfo() (*define.HostInfo, error) {
	// lets say OS, arch, number of cpus, amount of memory, maybe os distribution/version, hostname, kernel version, uptime
	mi, err := system.ReadMemInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading memory info")
	}

	hostDistributionInfo := r.GetHostDistributionInfo()

	kv, err := readKernelVersion()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading kernel version")
	}

	host, err := os.Hostname()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting hostname")
	}

	seccompProfilePath, err := DefaultSeccompPath()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting Seccomp profile path")
	}

	// Cgroups version
	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading cgroups mode")
	}

	// Get Map of all available controllers
	availableControllers, err := cgroups.GetAvailableControllers(nil, unified)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting available cgroup controllers")
	}
	cpuUtil, err := getCPUUtilization()
	if err != nil {
		return nil, err
	}
	info := define.HostInfo{
		Arch:              runtime.GOARCH,
		BuildahVersion:    buildah.Version,
		CgroupManager:     r.config.Engine.CgroupManager,
		CgroupControllers: availableControllers,
		Linkmode:          linkmode.Linkmode(),
		CPUs:              runtime.NumCPU(),
		CPUUtilization:    cpuUtil,
		Distribution:      hostDistributionInfo,
		LogDriver:         r.config.Containers.LogDriver,
		EventLogger:       r.eventer.String(),
		Hostname:          host,
		IDMappings:        define.IDMappings{},
		Kernel:            kv,
		MemFree:           mi.MemFree,
		MemTotal:          mi.MemTotal,
		NetworkBackend:    r.config.Network.NetworkBackend,
		OS:                runtime.GOOS,
		Security: define.SecurityInfo{
			AppArmorEnabled:     apparmor.IsEnabled(),
			DefaultCapabilities: strings.Join(r.config.Containers.DefaultCapabilities, ","),
			Rootless:            rootless.IsRootless(),
			SECCOMPEnabled:      seccomp.IsEnabled(),
			SECCOMPProfilePath:  seccompProfilePath,
			SELinuxEnabled:      selinux.GetEnabled(),
		},
		Slirp4NetNS: define.SlirpInfo{},
		SwapFree:    mi.SwapFree,
		SwapTotal:   mi.SwapTotal,
	}

	cgroupVersion := "v1"
	if unified {
		cgroupVersion = "v2"
	}
	info.CgroupsVersion = cgroupVersion

	slirp4netnsPath := r.config.Engine.NetworkCmdPath
	if slirp4netnsPath == "" {
		slirp4netnsPath, _ = exec.LookPath("slirp4netns")
	}
	if slirp4netnsPath != "" {
		version, err := programVersion(slirp4netnsPath)
		if err != nil {
			logrus.Warnf("Failed to retrieve program version for %s: %v", slirp4netnsPath, err)
		}
		program := define.SlirpInfo{
			Executable: slirp4netnsPath,
			Package:    packageVersion(slirp4netnsPath),
			Version:    version,
		}
		info.Slirp4NetNS = program
	}

	if rootless.IsRootless() {
		uidmappings, err := rootless.ReadMappingsProc("/proc/self/uid_map")
		if err != nil {
			return nil, errors.Wrapf(err, "error reading uid mappings")
		}
		gidmappings, err := rootless.ReadMappingsProc("/proc/self/gid_map")
		if err != nil {
			return nil, errors.Wrapf(err, "error reading gid mappings")
		}
		idmappings := define.IDMappings{
			GIDMap: gidmappings,
			UIDMap: uidmappings,
		}
		info.IDMappings = idmappings
	}

	conmonInfo, ociruntimeInfo, err := r.defaultOCIRuntime.RuntimeInfo()
	if err != nil {
		logrus.Errorf("Getting info on OCI runtime %s: %v", r.defaultOCIRuntime.Name(), err)
	} else {
		info.Conmon = conmonInfo
		info.OCIRuntime = ociruntimeInfo
	}

	up, err := readUptime()
	if err != nil {
		return nil, errors.Wrapf(err, "error reading up time")
	}
	// Convert uptime in seconds to a human-readable format
	upSeconds := up + "s"
	upDuration, err := time.ParseDuration(upSeconds)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing system uptime")
	}

	// TODO Isn't there a simple lib for this, something like humantime?
	hoursFound := false
	var timeBuffer bytes.Buffer
	var hoursBuffer bytes.Buffer
	for _, elem := range upDuration.String() {
		timeBuffer.WriteRune(elem)
		if elem == 'h' || elem == 'm' {
			timeBuffer.WriteRune(' ')
			if elem == 'h' {
				hoursFound = true
			}
		}
		if !hoursFound {
			hoursBuffer.WriteRune(elem)
		}
	}

	info.Uptime = timeBuffer.String()
	if hoursFound {
		hours, err := strconv.ParseFloat(hoursBuffer.String(), 64)
		if err == nil {
			days := hours / 24
			info.Uptime = fmt.Sprintf("%s (Approximately %.2f days)", info.Uptime, days)
		}
	}

	return &info, nil
}

func (r *Runtime) getContainerStoreInfo() (define.ContainerStore, error) {
	var (
		paused, running, stopped int
	)
	cs := define.ContainerStore{}
	cons, err := r.GetAllContainers()
	if err != nil {
		return cs, err
	}
	cs.Number = len(cons)
	for _, con := range cons {
		state, err := con.State()
		if err != nil {
			if errors.Cause(err) == define.ErrNoSuchCtr {
				// container was probably removed
				cs.Number--
				continue
			}
			return cs, err
		}
		switch state {
		case define.ContainerStateRunning:
			running++
		case define.ContainerStatePaused:
			paused++
		default:
			stopped++
		}
	}
	cs.Paused = paused
	cs.Stopped = stopped
	cs.Running = running
	return cs, nil
}

// top-level "store" info
func (r *Runtime) storeInfo() (*define.StoreInfo, error) {
	// lets say storage driver in use, number of images, number of containers
	configFile, err := storage.DefaultConfigFile(rootless.IsRootless())
	if err != nil {
		return nil, err
	}
	images, err := r.store.Images()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting number of images")
	}
	conInfo, err := r.getContainerStoreInfo()
	if err != nil {
		return nil, err
	}
	imageInfo := define.ImageStore{Number: len(images)}

	var grStats syscall.Statfs_t
	if err := syscall.Statfs(r.store.GraphRoot(), &grStats); err != nil {
		return nil, errors.Wrapf(err, "unable to collect graph root usasge for %q", r.store.GraphRoot())
	}
	allocated := uint64(grStats.Bsize) * grStats.Blocks
	info := define.StoreInfo{
		ImageStore:         imageInfo,
		ImageCopyTmpDir:    os.Getenv("TMPDIR"),
		ContainerStore:     conInfo,
		GraphRoot:          r.store.GraphRoot(),
		GraphRootAllocated: allocated,
		GraphRootUsed:      allocated - (uint64(grStats.Bsize) * grStats.Bfree),
		RunRoot:            r.store.RunRoot(),
		GraphDriverName:    r.store.GraphDriverName(),
		GraphOptions:       nil,
		VolumePath:         r.config.Engine.VolumePath,
		ConfigFile:         configFile,
	}

	graphOptions := map[string]interface{}{}
	for _, o := range r.store.GraphOptions() {
		split := strings.SplitN(o, "=", 2)
		if strings.HasSuffix(split[0], "mount_program") {
			version, err := programVersion(split[1])
			if err != nil {
				logrus.Warnf("Failed to retrieve program version for %s: %v", split[1], err)
			}
			program := map[string]interface{}{}
			program["Executable"] = split[1]
			program["Version"] = version
			program["Package"] = packageVersion(split[1])
			graphOptions[split[0]] = program
		} else {
			graphOptions[split[0]] = split[1]
		}
	}
	info.GraphOptions = graphOptions

	statusPairs, err := r.store.Status()
	if err != nil {
		return nil, err
	}
	status := map[string]string{}
	for _, pair := range statusPairs {
		status[pair[0]] = pair[1]
	}
	info.GraphStatus = status
	return &info, nil
}

func readKernelVersion() (string, error) {
	buf, err := ioutil.ReadFile("/proc/version")
	if err != nil {
		return "", err
	}
	f := bytes.Fields(buf)
	if len(f) < 3 {
		return string(bytes.TrimSpace(buf)), nil
	}
	return string(f[2]), nil
}

func readUptime() (string, error) {
	buf, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return "", err
	}
	f := bytes.Fields(buf)
	if len(f) < 1 {
		return "", fmt.Errorf("invalid uptime")
	}
	return string(f[0]), nil
}

// GetHostDistributionInfo returns a map containing the host's distribution and version
func (r *Runtime) GetHostDistributionInfo() define.DistributionInfo {
	// Populate values in case we cannot find the values
	// or the file
	dist := define.DistributionInfo{
		Distribution: "unknown",
		Version:      "unknown",
	}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return dist
	}
	defer f.Close()

	l := bufio.NewScanner(f)
	for l.Scan() {
		if strings.HasPrefix(l.Text(), "ID=") {
			dist.Distribution = strings.TrimPrefix(l.Text(), "ID=")
		}
		if strings.HasPrefix(l.Text(), "VARIANT_ID=") {
			dist.Variant = strings.Trim(strings.TrimPrefix(l.Text(), "VARIANT_ID="), "\"")
		}
		if strings.HasPrefix(l.Text(), "VERSION_ID=") {
			dist.Version = strings.Trim(strings.TrimPrefix(l.Text(), "VERSION_ID="), "\"")
		}
		if strings.HasPrefix(l.Text(), "VERSION_CODENAME=") {
			dist.Codename = strings.Trim(strings.TrimPrefix(l.Text(), "VERSION_CODENAME="), "\"")
		}
	}
	return dist
}

// getCPUUtilization Returns a CPUUsage object that summarizes CPU
// usage for userspace, system, and idle time.
func getCPUUtilization() (*define.CPUUsage, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	// Read firt line of /proc/stat
	for scanner.Scan() {
		break
	}
	// column 1 is user, column 3 is system, column 4 is idle
	stats := strings.Split(scanner.Text(), " ")
	return statToPercent(stats)
}

func statToPercent(stats []string) (*define.CPUUsage, error) {
	// There is always an extra space between cpu and the first metric
	userTotal, err := strconv.ParseFloat(stats[2], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse user value %q", stats[1])
	}
	systemTotal, err := strconv.ParseFloat(stats[4], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse system value %q", stats[3])
	}
	idleTotal, err := strconv.ParseFloat(stats[5], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse idle value %q", stats[4])
	}
	total := userTotal + systemTotal + idleTotal
	s := define.CPUUsage{
		UserPercent:   math.Round((userTotal/total*100)*100) / 100,
		SystemPercent: math.Round((systemTotal/total*100)*100) / 100,
		IdlePercent:   math.Round((idleTotal/total*100)*100) / 100,
	}
	return &s, nil
}
