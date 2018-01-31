package main

import (
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/docker/daemon/caps"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/profiles/seccomp"
	"github.com/docker/go-units"
	"github.com/opencontainers/runc/libcontainer/devices"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	ann "github.com/projectatomic/libpod/pkg/annotations"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const cpuPeriod = 100000

func blockAccessToKernelFilesystems(config *createConfig, g *generate.Generator) {
	if !config.Privileged {
		for _, mp := range []string{
			"/proc/kcore",
			"/proc/latency_stats",
			"/proc/timer_list",
			"/proc/timer_stats",
			"/proc/sched_debug",
			"/proc/scsi",
			"/sys/firmware",
		} {
			g.AddLinuxMaskedPaths(mp)
		}

		for _, rp := range []string{
			"/proc/asound",
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger",
		} {
			g.AddLinuxReadonlyPaths(rp)
		}
	}
}

func addPidNS(config *createConfig, g *generate.Generator) error {
	pidMode := config.PidMode
	if pidMode.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.PIDNamespace))
	}
	if pidMode.IsContainer() {
		logrus.Debug("using container pidmode")
	}
	return nil
}

func addNetNS(config *createConfig, g *generate.Generator) error {
	netMode := config.NetMode
	if netMode.IsHost() {
		logrus.Debug("Using host netmode")
		return g.RemoveLinuxNamespace(spec.NetworkNamespace)
	} else if netMode.IsNone() {
		logrus.Debug("Using none netmode")
		return nil
	} else if netMode.IsBridge() {
		logrus.Debug("Using bridge netmode")
		return nil
	} else if netMode.IsContainer() {
		logrus.Debug("Using container netmode")
	} else {
		return errors.Errorf("unknown network mode")
	}
	return nil
}

func addUTSNS(config *createConfig, g *generate.Generator) error {
	utsMode := config.UtsMode
	if utsMode.IsHost() {
		return g.RemoveLinuxNamespace(spec.UTSNamespace)
	}
	return nil
}

func addIpcNS(config *createConfig, g *generate.Generator) error {
	ipcMode := config.IpcMode
	if ipcMode.IsHost() {
		return g.RemoveLinuxNamespace(spec.IPCNamespace)
	}
	if ipcMode.IsContainer() {
		logrus.Debug("Using container ipcmode")
	}

	return nil
}

func addRlimits(config *createConfig, g *generate.Generator) error {
	var (
		ul  *units.Ulimit
		err error
	)

	for _, u := range config.Resources.Ulimit {
		if ul, err = units.ParseUlimit(u); err != nil {
			return errors.Wrapf(err, "ulimit option %q requires name=SOFT:HARD, failed to be parsed", u)
		}

		g.AddProcessRlimits("RLIMIT_"+strings.ToUpper(ul.Name), uint64(ul.Hard), uint64(ul.Soft))
	}
	return nil
}

func setupCapabilities(config *createConfig, configSpec *spec.Spec) error {
	var err error
	var caplist []string
	if config.Privileged {
		caplist = caps.GetAllCapabilities()
	} else {
		caplist, err = caps.TweakCapabilities(configSpec.Process.Capabilities.Bounding, config.CapAdd, config.CapDrop)
		if err != nil {
			return err
		}
	}

	configSpec.Process.Capabilities.Bounding = caplist
	configSpec.Process.Capabilities.Permitted = caplist
	configSpec.Process.Capabilities.Inheritable = caplist
	configSpec.Process.Capabilities.Effective = caplist
	return nil
}

func addDevice(g *generate.Generator, device string) error {
	dev, err := devices.DeviceFromPath(device, "rwm")
	if err != nil {
		return errors.Wrapf(err, "%s is not a valid device", device)
	}
	linuxdev := spec.LinuxDevice{
		Path:     dev.Path,
		Type:     string(dev.Type),
		Major:    dev.Major,
		Minor:    dev.Minor,
		FileMode: &dev.FileMode,
		UID:      &dev.Uid,
		GID:      &dev.Gid,
	}
	g.AddDevice(linuxdev)
	g.AddLinuxResourcesDevice(true, string(dev.Type), &dev.Major, &dev.Minor, dev.Permissions)
	return nil
}

// Parses information needed to create a container into an OCI runtime spec
func createConfigToOCISpec(config *createConfig) (*spec.Spec, error) {
	cgroupPerm := "ro"
	g := generate.New()
	if config.Privileged {
		cgroupPerm = "rw"
		g.RemoveMount("/sys")
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"nosuid", "noexec", "nodev", "rw"},
		}
		g.AddMount(sysMnt)
	}
	cgroupMnt := spec.Mount{
		Destination: "/sys/fs/cgroup",
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"nosuid", "noexec", "nodev", "relatime", cgroupPerm},
	}
	g.AddMount(cgroupMnt)
	g.SetProcessCwd(config.WorkDir)
	g.SetProcessArgs(config.Command)
	g.SetProcessTerminal(config.Tty)
	for _, gid := range config.GroupAdd {
		g.AddProcessAdditionalGid(gid)
	}
	for key, val := range config.GetAnnotations() {
		g.AddAnnotation(key, val)
	}
	g.SetRootReadonly(config.ReadOnlyRootfs)
	g.SetHostname(config.Hostname)
	if config.Hostname != "" {
		g.AddProcessEnv("HOSTNAME", config.Hostname)
	}
	for sysctlKey, sysctlVal := range config.Sysctl {
		g.AddLinuxSysctl(sysctlKey, sysctlVal)
	}

	// RESOURCES - MEMORY
	if config.Resources.Memory != 0 {
		g.SetLinuxResourcesMemoryLimit(config.Resources.Memory)
	}
	if config.Resources.MemoryReservation != 0 {
		g.SetLinuxResourcesMemoryReservation(config.Resources.MemoryReservation)
	}
	if config.Resources.MemorySwap != 0 {
		g.SetLinuxResourcesMemorySwap(config.Resources.MemorySwap)
	}
	if config.Resources.KernelMemory != 0 {
		g.SetLinuxResourcesMemoryKernel(config.Resources.KernelMemory)
	}
	if config.Resources.MemorySwappiness != -1 {
		g.SetLinuxResourcesMemorySwappiness(uint64(config.Resources.MemorySwappiness))
	}
	g.SetLinuxResourcesMemoryDisableOOMKiller(config.Resources.DisableOomKiller)
	g.SetProcessOOMScoreAdj(config.Resources.OomScoreAdj)

	// RESOURCES - CPU
	if config.Resources.CPUShares != 0 {
		g.SetLinuxResourcesCPUShares(config.Resources.CPUShares)
	}
	if config.Resources.CPUQuota != 0 {
		g.SetLinuxResourcesCPUQuota(config.Resources.CPUQuota)
	}
	if config.Resources.CPUPeriod != 0 {
		g.SetLinuxResourcesCPUPeriod(config.Resources.CPUPeriod)
	}
	if config.Resources.CPUs != 0 {
		g.SetLinuxResourcesCPUPeriod(cpuPeriod)
		g.SetLinuxResourcesCPUQuota(int64(config.Resources.CPUs * cpuPeriod))
	}
	if config.Resources.CPURtRuntime != 0 {
		g.SetLinuxResourcesCPURealtimeRuntime(config.Resources.CPURtRuntime)
	}
	if config.Resources.CPURtPeriod != 0 {
		g.SetLinuxResourcesCPURealtimePeriod(config.Resources.CPURtPeriod)
	}
	if config.Resources.CPUsetCPUs != "" {
		g.SetLinuxResourcesCPUCpus(config.Resources.CPUsetCPUs)
	}
	if config.Resources.CPUsetMems != "" {
		g.SetLinuxResourcesCPUMems(config.Resources.CPUsetMems)
	}

	// Devices
	for _, device := range config.Devices {
		if err := addDevice(&g, device); err != nil {
			return nil, err
		}
	}

	// SECURITY OPTS
	g.SetProcessNoNewPrivileges(config.NoNewPrivileges)
	g.SetProcessApparmorProfile(config.ApparmorProfile)
	g.SetProcessSelinuxLabel(config.ProcessLabel)
	g.SetLinuxMountLabel(config.MountLabel)
	blockAccessToKernelFilesystems(config, &g)

	// RESOURCES - PIDS
	if config.Resources.PidsLimit != 0 {
		g.SetLinuxResourcesPidsLimit(config.Resources.PidsLimit)
	}

	for _, i := range config.Tmpfs {
		// Default options if nothing passed
		options := []string{"rw", "noexec", "nosuid", "nodev", "size=65536k"}
		spliti := strings.SplitN(i, ":", 2)
		if len(spliti) > 1 {
			if _, _, err := mount.ParseTmpfsOptions(spliti[1]); err != nil {
				return nil, err
			}
			options = strings.Split(spliti[1], ",")
		}
		tmpfsMnt := spec.Mount{
			Destination: spliti[0],
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     append(options, "tmpcopyup"),
		}
		g.AddMount(tmpfsMnt)
	}

	for name, val := range config.Env {
		g.AddProcessEnv(name, val)
	}

	if err := addRlimits(config, &g); err != nil {
		return nil, err
	}

	if err := addPidNS(config, &g); err != nil {
		return nil, err
	}

	if err := addNetNS(config, &g); err != nil {
		return nil, err
	}

	if err := addUTSNS(config, &g); err != nil {
		return nil, err
	}

	if err := addIpcNS(config, &g); err != nil {
		return nil, err
	}
	configSpec := g.Spec()

	// HANDLE CAPABILITIES
	// NOTE: Must happen before SECCOMP
	if err := setupCapabilities(config, configSpec); err != nil {
		return nil, err
	}

	// HANDLE SECCOMP
	if config.SeccompProfilePath != "unconfined" {
		if config.SeccompProfilePath != "" {
			seccompProfile, err := ioutil.ReadFile(config.SeccompProfilePath)
			if err != nil {
				return nil, errors.Wrapf(err, "opening seccomp profile (%s) failed", config.SeccompProfilePath)
			}
			seccompConfig, err := seccomp.LoadProfile(string(seccompProfile), configSpec)
			if err != nil {
				return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
			}
			configSpec.Linux.Seccomp = seccompConfig
		} else {
			seccompConfig, err := seccomp.GetDefaultProfile(configSpec)
			if err != nil {
				return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
			}
			configSpec.Linux.Seccomp = seccompConfig
		}
	}

	// BIND MOUNTS
	mounts, err := config.GetVolumeMounts()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting volume mounts")
	}
	configSpec.Mounts = append(configSpec.Mounts, mounts...)
	for _, mount := range configSpec.Mounts {
		for _, opt := range mount.Options {
			switch opt {
			case "private", "rprivate", "slave", "rslave", "shared", "rshared":
				if err := g.SetLinuxRootPropagation(opt); err != nil {
					return nil, errors.Wrapf(err, "error setting root propagation for %q", mount.Destination)
				}
			}
		}
	}

	// BLOCK IO
	blkio, err := config.CreateBlockIO()
	if err != nil {
		return nil, errors.Wrapf(err, "error creating block io")
	}
	if blkio != nil {
		configSpec.Linux.Resources.BlockIO = blkio
	}

	/*
			Hooks: &configSpec.Hooks{},
			//Annotations
				Resources: &configSpec.LinuxResources{
					BlockIO: &blkio,
					//HugepageLimits:
					Network: &configSpec.LinuxNetwork{
					// ClassID *uint32
					// Priorites []LinuxInterfacePriority
					},
				},
				//CgroupsPath:
				//Namespaces: []LinuxNamespace
				// DefaultAction:
				// Architectures
				// Syscalls:
				},
				// RootfsPropagation
				// MaskedPaths
				// ReadonlyPaths:
				// IntelRdt
			},
		}
	*/
	return configSpec, nil
}

func (c *createConfig) CreateBlockIO() (*spec.LinuxBlockIO, error) {
	bio := &spec.LinuxBlockIO{}
	bio.Weight = &c.Resources.BlkioWeight
	if len(c.Resources.BlkioWeightDevice) > 0 {
		var lwds []spec.LinuxWeightDevice
		for _, i := range c.Resources.BlkioWeightDevice {
			wd, err := validateweightDevice(i)
			if err != nil {
				return bio, errors.Wrapf(err, "invalid values for blkio-weight-device")
			}
			wdStat := getStatFromPath(wd.path)
			lwd := spec.LinuxWeightDevice{
				Weight: &wd.weight,
			}
			lwd.Major = int64(unix.Major(wdStat.Rdev))
			lwd.Minor = int64(unix.Minor(wdStat.Rdev))
			lwds = append(lwds, lwd)
		}
		bio.WeightDevice = lwds
	}
	if len(c.Resources.DeviceReadBps) > 0 {
		readBps, err := makeThrottleArray(c.Resources.DeviceReadBps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadBpsDevice = readBps
	}
	if len(c.Resources.DeviceWriteBps) > 0 {
		writeBpds, err := makeThrottleArray(c.Resources.DeviceWriteBps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteBpsDevice = writeBpds
	}
	if len(c.Resources.DeviceReadIOps) > 0 {
		readIOps, err := makeThrottleArray(c.Resources.DeviceReadIOps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadIOPSDevice = readIOps
	}
	if len(c.Resources.DeviceWriteIOps) > 0 {
		writeIOps, err := makeThrottleArray(c.Resources.DeviceWriteIOps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteIOPSDevice = writeIOps
	}
	return bio, nil
}

// GetAnnotations returns the all the annotations for the container
func (c *createConfig) GetAnnotations() map[string]string {
	a := getDefaultAnnotations()
	// TODO - Which annotations do we want added by default
	// TODO - This should be added to the DB long term
	if c.Tty {
		a["io.kubernetes.cri-o.TTY"] = "true"
	}
	return a
}

func getDefaultAnnotations() map[string]string {
	var annotations map[string]string
	annotations = make(map[string]string)
	annotations[ann.Annotations] = ""
	annotations[ann.ContainerID] = ""
	annotations[ann.ContainerName] = ""
	annotations[ann.ContainerType] = ""
	annotations[ann.Created] = ""
	annotations[ann.HostName] = ""
	annotations[ann.IP] = ""
	annotations[ann.Image] = ""
	annotations[ann.ImageName] = ""
	annotations[ann.ImageRef] = ""
	annotations[ann.KubeName] = ""
	annotations[ann.Labels] = ""
	annotations[ann.LogPath] = ""
	annotations[ann.Metadata] = ""
	annotations[ann.Name] = ""
	annotations[ann.PrivilegedRuntime] = ""
	annotations[ann.ResolvPath] = ""
	annotations[ann.HostnamePath] = ""
	annotations[ann.SandboxID] = ""
	annotations[ann.SandboxName] = ""
	annotations[ann.ShmPath] = ""
	annotations[ann.MountPoint] = ""
	annotations[ann.TrustedSandbox] = ""
	annotations[ann.TTY] = "false"
	annotations[ann.Stdin] = ""
	annotations[ann.StdinOnce] = ""
	annotations[ann.Volumes] = ""

	return annotations
}

//GetVolumeMounts takes user provided input for bind mounts and creates Mount structs
func (c *createConfig) GetVolumeMounts() ([]spec.Mount, error) {
	var m []spec.Mount
	var options []string
	for _, i := range c.Volumes {
		// We need to handle SELinux options better here, specifically :Z
		spliti := strings.Split(i, ":")
		if len(spliti) > 2 {
			options = strings.Split(spliti[2], ",")
		}
		options = append(options, "rbind")
		var foundrw, foundro, foundz, foundZ bool
		var rootProp string
		for _, opt := range options {
			switch opt {
			case "rw":
				foundrw = true
			case "ro":
				foundro = true
			case "z":
				foundz = true
			case "Z":
				foundZ = true
			case "private", "rprivate", "slave", "rslave", "shared", "rshared":
				rootProp = opt
			}
		}
		if !foundrw && !foundro {
			options = append(options, "rw")
		}
		if foundz {
			if err := label.Relabel(spliti[0], c.MountLabel, true); err != nil {
				return nil, errors.Wrapf(err, "relabel failed %q", spliti[0])
			}
		}
		if foundZ {
			if err := label.Relabel(spliti[0], c.MountLabel, false); err != nil {
				return nil, errors.Wrapf(err, "relabel failed %q", spliti[0])
			}
		}
		if rootProp == "" {
			options = append(options, "private")
		}

		m = append(m, spec.Mount{
			Destination: spliti[1],
			Type:        string(TypeBind),
			Source:      spliti[0],
			Options:     options,
		})
	}
	return m, nil
}

//GetTmpfsMounts takes user provided input for Tmpfs mounts and creates Mount structs
func (c *createConfig) GetTmpfsMounts() []spec.Mount {
	var m []spec.Mount
	for _, i := range c.Tmpfs {
		// Default options if nothing passed
		options := []string{"rw", "noexec", "nosuid", "nodev", "size=65536k"}
		spliti := strings.Split(i, ":")
		destPath := spliti[0]
		if len(spliti) > 1 {
			options = strings.Split(spliti[1], ",")
		}
		m = append(m, spec.Mount{
			Destination: destPath,
			Type:        string(TypeTmpfs),
			Options:     options,
			Source:      string(TypeTmpfs),
		})
	}
	return m
}

func (c *createConfig) GetContainerCreateOptions() ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption
	var portBindings []ocicni.PortMapping
	var err error

	// Uncomment after talking to mheon about unimplemented funcs
	// options = append(options, libpod.WithLabels(c.labels))

	if c.Interactive {
		options = append(options, libpod.WithStdin())
	}
	if c.Name != "" {
		logrus.Debugf("appending name %s", c.Name)
		options = append(options, libpod.WithName(c.Name))
	}

	// TODO deal with ports defined in image metadata
	if len(c.PortBindings) > 0 || len(c.ExposedPorts) > 0 {
		portBindings, err = c.CreatePortBindings()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create port bindings")
		}
	}

	if c.NetMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.NetMode.ConnectedContainer())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.NetMode.ConnectedContainer())
		}
		options = append(options, libpod.WithNetNSFrom(connectedCtr))
	} else if !c.NetMode.IsHost() && !c.NetMode.IsNone() {
		options = append(options, libpod.WithNetNS([]ocicni.PortMapping{}))
		options = append(options, libpod.WithNetNS(portBindings))
	}

	if c.PidMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.PidMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.PidMode.Container())
		}

		options = append(options, libpod.WithPIDNSFrom(connectedCtr))
	}
	if c.IpcMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.IpcMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.IpcMode.Container())
		}

		options = append(options, libpod.WithIPCNSFrom(connectedCtr))
	}

	options = append(options, libpod.WithStopSignal(c.StopSignal))
	options = append(options, libpod.WithStopTimeout(c.StopTimeout))
	if len(c.DNSSearch) > 0 {
		options = append(options, libpod.WithDNSSearch(c.DNSSearch))
	}
	if len(c.DNSServers) > 0 {
		options = append(options, libpod.WithDNS(c.DNSServers))
	}
	if len(c.DNSOpt) > 0 {
		options = append(options, libpod.WithDNSOption(c.DNSOpt))
	}
	if len(c.HostAdd) > 0 {
		options = append(options, libpod.WithHosts(c.HostAdd))
	}

	options = append(options, libpod.WithPrivileged(c.Privileged))
	return options, nil
}

func getStatFromPath(path string) unix.Stat_t {
	s := unix.Stat_t{}
	_ = unix.Stat(path, &s)
	return s
}

func makeThrottleArray(throttleInput []string) ([]spec.LinuxThrottleDevice, error) {
	var ltds []spec.LinuxThrottleDevice
	for _, i := range throttleInput {
		t, err := validateBpsDevice(i)
		if err != nil {
			return []spec.LinuxThrottleDevice{}, err
		}
		ltd := spec.LinuxThrottleDevice{}
		ltd.Rate = t.rate
		ltdStat := getStatFromPath(t.path)
		ltd.Major = int64(unix.Major(ltdStat.Rdev))
		ltd.Minor = int64(unix.Major(ltdStat.Rdev))
		ltds = append(ltds, ltd)
	}
	return ltds, nil
}

// CreatePortBindings iterates ports mappings and exposed ports into a format CNI understands
func (c *createConfig) CreatePortBindings() ([]ocicni.PortMapping, error) {
	var portBindings []ocicni.PortMapping
	for containerPb, hostPb := range c.PortBindings {
		var pm ocicni.PortMapping
		pm.ContainerPort = int32(containerPb.Int())
		for _, i := range hostPb {
			var hostPort int
			var err error
			pm.HostIP = i.HostIP
			if i.HostPort == "" {
				hostPort = containerPb.Int()
			} else {
				hostPort, err = strconv.Atoi(i.HostPort)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to convert host port to integer")
				}
			}

			pm.HostPort = int32(hostPort)
			// CNI requires us to make both udp and tcp structs
			pm.Protocol = "udp"
			portBindings = append(portBindings, pm)
			pm.Protocol = "tcp"
			portBindings = append(portBindings, pm)
		}
	}
	for j := range c.ExposedPorts {
		var expose ocicni.PortMapping
		expose.HostPort = int32(j.Int())
		expose.ContainerPort = int32(j.Int())
		// CNI requires us to make both udp and tcp structs
		expose.Protocol = "udp"
		portBindings = append(portBindings, expose)
		expose.Protocol = "tcp"
		portBindings = append(portBindings, expose)
	}
	return portBindings, nil
}
