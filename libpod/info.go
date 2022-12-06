package libpod

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/buildah/pkg/util"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/linkmode"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/system"
	"github.com/sirupsen/logrus"
)

// Info returns the store and host information
func (r *Runtime) info() (*define.Info, error) {
	info := define.Info{}
	versionInfo, err := define.GetVersion()
	if err != nil {
		return nil, fmt.Errorf("getting version info: %w", err)
	}
	info.Version = versionInfo
	// get host information
	hostInfo, err := r.hostInfo()
	if err != nil {
		return nil, fmt.Errorf("getting host info: %w", err)
	}
	info.Host = hostInfo

	// get store information
	storeInfo, err := r.storeInfo()
	if err != nil {
		return nil, fmt.Errorf("getting store info: %w", err)
	}
	info.Store = storeInfo
	registries := make(map[string]interface{})

	sys := r.SystemContext()
	data, err := sysregistriesv2.GetRegistries(sys)
	if err != nil {
		return nil, fmt.Errorf("getting registries: %w", err)
	}
	for _, reg := range data {
		registries[reg.Prefix] = reg
	}
	regs, err := sysregistriesv2.UnqualifiedSearchRegistries(sys)
	if err != nil {
		return nil, fmt.Errorf("getting registries: %w", err)
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
		return nil, fmt.Errorf("reading memory info: %w", err)
	}

	hostDistributionInfo := r.GetHostDistributionInfo()

	kv, err := util.ReadKernelVersion()
	if err != nil {
		return nil, fmt.Errorf("reading kernel version: %w", err)
	}

	host, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("getting hostname: %w", err)
	}

	cpuUtil, err := getCPUUtilization()
	if err != nil {
		return nil, err
	}
	info := define.HostInfo{
		Arch:           runtime.GOARCH,
		BuildahVersion: buildah.Version,
		Linkmode:       linkmode.Linkmode(),
		CPUs:           runtime.NumCPU(),
		CPUUtilization: cpuUtil,
		Distribution:   hostDistributionInfo,
		LogDriver:      r.config.Containers.LogDriver,
		EventLogger:    r.eventer.String(),
		Hostname:       host,
		Kernel:         kv,
		MemFree:        mi.MemFree,
		MemTotal:       mi.MemTotal,
		NetworkBackend: r.config.Network.NetworkBackend,
		OS:             runtime.GOOS,
		SwapFree:       mi.SwapFree,
		SwapTotal:      mi.SwapTotal,
	}
	if err := r.setPlatformHostInfo(&info); err != nil {
		return nil, err
	}

	conmonInfo, ociruntimeInfo, err := r.defaultOCIRuntime.RuntimeInfo()
	if err != nil {
		logrus.Errorf("Getting info on OCI runtime %s: %v", r.defaultOCIRuntime.Name(), err)
	} else {
		info.Conmon = conmonInfo
		info.OCIRuntime = ociruntimeInfo
	}

	duration, err := util.ReadUptime()
	if err != nil {
		return nil, fmt.Errorf("reading up time: %w", err)
	}

	uptime := struct {
		hours   float64
		minutes float64
		seconds float64
	}{
		hours:   duration.Truncate(time.Hour).Hours(),
		minutes: duration.Truncate(time.Minute).Minutes(),
		seconds: duration.Truncate(time.Second).Seconds(),
	}

	// Could not find a humanize-formatter for time.Duration
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%.0fh %.0fm %.2fs",
		uptime.hours,
		math.Mod(uptime.seconds, 3600)/60,
		math.Mod(uptime.seconds, 60),
	))
	if int64(uptime.hours) > 0 {
		buffer.WriteString(fmt.Sprintf(" (Approximately %.2f days)", uptime.hours/24))
	}
	info.Uptime = buffer.String()

	return &info, nil
}

func (r *Runtime) getContainerStoreInfo() (define.ContainerStore, error) {
	var paused, running, stopped int
	cs := define.ContainerStore{}
	cons, err := r.GetAllContainers()
	if err != nil {
		return cs, err
	}
	cs.Number = len(cons)
	for _, con := range cons {
		state, err := con.State()
		if err != nil {
			if errors.Is(err, define.ErrNoSuchCtr) {
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
		return nil, fmt.Errorf("getting number of images: %w", err)
	}
	conInfo, err := r.getContainerStoreInfo()
	if err != nil {
		return nil, err
	}
	imageInfo := define.ImageStore{Number: len(images)}

	var grStats syscall.Statfs_t
	if err := syscall.Statfs(r.store.GraphRoot(), &grStats); err != nil {
		return nil, fmt.Errorf("unable to collect graph root usasge for %q: %w", r.store.GraphRoot(), err)
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
		TransientStore:     r.store.TransientStore(),
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
