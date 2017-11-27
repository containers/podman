package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/docker/docker/daemon/caps"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/go-units"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	ann "github.com/projectatomic/libpod/pkg/annotations"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func blockAccessToKernelFilesystems(config *createConfig, g *generate.Generator) {
	if !config.privileged {
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
	pidMode := config.pidMode
	if pidMode.IsHost() {
		return g.RemoveLinuxNamespace(libpod.PIDNamespace)
	}
	if pidMode.IsContainer() {
		ctr, err := config.runtime.LookupContainer(pidMode.Container())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", pidMode.Container())
		}
		pid, err := ctr.PID()
		if err != nil {
			return errors.Wrapf(err, "Failed to get pid of container %q", pidMode.Container())
		}
		pidNsPath := fmt.Sprintf("/proc/%d/ns/pid", pid)
		if err := g.AddOrReplaceLinuxNamespace(libpod.PIDNamespace, pidNsPath); err != nil {
			return err
		}
	}
	return nil
}

func addNetNS(config *createConfig, g *generate.Generator) error {
	netMode := config.netMode
	if netMode.IsHost() {
		return g.RemoveLinuxNamespace(libpod.NetNamespace)
	}
	if netMode.IsNone() {
		return libpod.ErrNotImplemented
	}
	if netMode.IsBridge() {
		return libpod.ErrNotImplemented
	}
	if netMode.IsContainer() {
		ctr, err := config.runtime.LookupContainer(netMode.ConnectedContainer())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", netMode.ConnectedContainer())
		}
		pid, err := ctr.PID()
		if err != nil {
			return errors.Wrapf(err, "Failed to get pid of container %q", netMode.ConnectedContainer())
		}
		nsPath := fmt.Sprintf("/proc/%d/ns/net", pid)
		if err := g.AddOrReplaceLinuxNamespace(libpod.NetNamespace, nsPath); err != nil {
			return err
		}
	}
	return nil
}

func addUTSNS(config *createConfig, g *generate.Generator) error {
	utsMode := config.utsMode
	if utsMode.IsHost() {
		return g.RemoveLinuxNamespace(libpod.UTSNamespace)
	}
	return nil
}

func addIpcNS(config *createConfig, g *generate.Generator) error {
	ipcMode := config.ipcMode
	if ipcMode.IsHost() {
		return g.RemoveLinuxNamespace(libpod.IPCNamespace)
	}
	if ipcMode.IsContainer() {
		ctr, err := config.runtime.LookupContainer(ipcMode.Container())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", ipcMode.Container())
		}
		pid, err := ctr.PID()
		if err != nil {
			return errors.Wrapf(err, "Failed to get pid of container %q", ipcMode.Container())
		}
		nsPath := fmt.Sprintf("/proc/%d/ns/ipc", pid)
		if err := g.AddOrReplaceLinuxNamespace(libpod.IPCNamespace, nsPath); err != nil {
			return err
		}
	}

	return nil
}

func addRlimits(config *createConfig, g *generate.Generator) error {
	var (
		ul  *units.Ulimit
		err error
	)

	for _, u := range config.resources.ulimit {
		if ul, err = units.ParseUlimit(u); err != nil {
			return errors.Wrapf(err, "ulimit option %q requires name=SOFT:HARD, failed to be parsed", u)
		}

		g.AddProcessRlimits("RLIMIT_"+strings.ToUpper(ul.Name), uint64(ul.Soft), uint64(ul.Hard))
	}
	return nil
}

func setupCapabilities(config *createConfig, configSpec *spec.Spec) error {
	var err error
	var caplist []string
	if config.privileged {
		caplist = caps.GetAllCapabilities()
	} else {
		caplist, err = caps.TweakCapabilities(configSpec.Process.Capabilities.Bounding, config.capAdd, config.capDrop)
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

// Parses information needed to create a container into an OCI runtime spec
func createConfigToOCISpec(config *createConfig) (*spec.Spec, error) {
	g := generate.New()
	g.AddCgroupsMount("ro")
	g.SetProcessCwd(config.workDir)
	g.SetProcessArgs(config.command)
	g.SetProcessTerminal(config.tty)
	// User and Group must go together
	g.SetProcessUID(config.user)
	g.SetProcessGID(config.group)
	for _, gid := range config.groupAdd {
		g.AddProcessAdditionalGid(gid)
	}
	for key, val := range config.GetAnnotations() {
		g.AddAnnotation(key, val)
	}
	g.SetRootReadonly(config.readOnlyRootfs)
	g.SetHostname(config.hostname)
	if config.hostname != "" {
		g.AddProcessEnv("HOSTNAME", config.hostname)
	}

	for _, sysctl := range config.sysctl {
		s := strings.SplitN(sysctl, "=", 2)
		g.AddLinuxSysctl(s[0], s[1])
	}

	// RESOURCES - MEMORY
	if config.resources.memory != 0 {
		g.SetLinuxResourcesMemoryLimit(config.resources.memory)
	}
	if config.resources.memoryReservation != 0 {
		g.SetLinuxResourcesMemoryReservation(config.resources.memoryReservation)
	}
	if config.resources.memorySwap != 0 {
		g.SetLinuxResourcesMemorySwap(config.resources.memorySwap)
	}
	if config.resources.kernelMemory != 0 {
		g.SetLinuxResourcesMemoryKernel(config.resources.kernelMemory)
	}
	if config.resources.memorySwappiness != -1 {
		g.SetLinuxResourcesMemorySwappiness(uint64(config.resources.memorySwappiness))
	}
	g.SetLinuxResourcesMemoryDisableOOMKiller(config.resources.disableOomKiller)
	g.SetProcessOOMScoreAdj(config.resources.oomScoreAdj)

	// RESOURCES - CPU

	if config.resources.cpuShares != 0 {
		g.SetLinuxResourcesCPUShares(config.resources.cpuShares)
	}
	if config.resources.cpuQuota != 0 {
		g.SetLinuxResourcesCPUQuota(config.resources.cpuQuota)
	}
	if config.resources.cpuPeriod != 0 {
		g.SetLinuxResourcesCPUPeriod(config.resources.cpuPeriod)
	}
	if config.resources.cpuRtRuntime != 0 {
		g.SetLinuxResourcesCPURealtimeRuntime(config.resources.cpuRtRuntime)
	}
	if config.resources.cpuRtPeriod != 0 {
		g.SetLinuxResourcesCPURealtimePeriod(config.resources.cpuRtPeriod)
	}
	if config.resources.cpus != "" {
		g.SetLinuxResourcesCPUCpus(config.resources.cpus)
	}
	if config.resources.cpusetMems != "" {
		g.SetLinuxResourcesCPUMems(config.resources.cpusetMems)
	}

	// SECURITY OPTS
	g.SetProcessNoNewPrivileges(config.noNewPrivileges)
	g.SetProcessApparmorProfile(config.apparmorProfile)
	g.SetProcessSelinuxLabel(config.processLabel)
	g.SetLinuxMountLabel(config.mountLabel)
	blockAccessToKernelFilesystems(config, &g)

	// RESOURCES - PIDS
	if config.resources.pidsLimit != 0 {
		g.SetLinuxResourcesPidsLimit(config.resources.pidsLimit)
	}

	for _, i := range config.tmpfs {
		options := []string{"rw", "noexec", "nosuid", "nodev", "size=65536k"}
		spliti := strings.SplitN(i, ":", 2)
		if len(spliti) > 1 {
			if _, _, err := mount.ParseTmpfsOptions(spliti[1]); err != nil {
				return nil, err
			}
			options = strings.Split(spliti[1], ",")
		}
		// Default options if nothing passed
		g.AddTmpfsMount(spliti[0], options)
	}

	for name, val := range config.env {
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

	if config.seccompProfilePath != "" && config.seccompProfilePath != "unconfined" {
		seccompProfile, err := ioutil.ReadFile(config.seccompProfilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "opening seccomp profile (%s) failed", config.seccompProfilePath)
		}
		var seccompConfig spec.LinuxSeccomp
		if err := json.Unmarshal(seccompProfile, &seccompConfig); err != nil {
			return nil, errors.Wrapf(err, "decoding seccomp profile (%s) failed", config.seccompProfilePath)
		}
		configSpec.Linux.Seccomp = &seccompConfig
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

	// HANDLE CAPABILITIES
	if err := setupCapabilities(config, configSpec); err != nil {
		return nil, err
	}

	/*
			Hooks: &configSpec.Hooks{},
			//Annotations
				Resources: &configSpec.LinuxResources{
					Devices: config.GetDefaultDevices(),
					BlockIO: &blkio,
					//HugepageLimits:
					Network: &configSpec.LinuxNetwork{
					// ClassID *uint32
					// Priorites []LinuxInterfacePriority
					},
				},
				//CgroupsPath:
				//Namespaces: []LinuxNamespace
				//Devices
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

func (c *createConfig) CreateBlockIO() (spec.LinuxBlockIO, error) {
	bio := spec.LinuxBlockIO{}
	bio.Weight = &c.resources.blkioWeight
	if len(c.resources.blkioWeightDevice) > 0 {
		var lwds []spec.LinuxWeightDevice
		for _, i := range c.resources.blkioWeightDevice {
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
	}
	if len(c.resources.deviceReadBps) > 0 {
		readBps, err := makeThrottleArray(c.resources.deviceReadBps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadBpsDevice = readBps
	}
	if len(c.resources.deviceWriteBps) > 0 {
		writeBpds, err := makeThrottleArray(c.resources.deviceWriteBps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteBpsDevice = writeBpds
	}
	if len(c.resources.deviceReadIOps) > 0 {
		readIOps, err := makeThrottleArray(c.resources.deviceReadIOps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadIOPSDevice = readIOps
	}
	if len(c.resources.deviceWriteIOps) > 0 {
		writeIOps, err := makeThrottleArray(c.resources.deviceWriteIOps)
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
	if c.tty {
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
	for _, i := range c.volumes {
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
			if err := label.Relabel(spliti[0], c.mountLabel, true); err != nil {
				return nil, errors.Wrapf(err, "relabel failed %q", spliti[0])
			}
		}
		if foundZ {
			if err := label.Relabel(spliti[0], c.mountLabel, false); err != nil {
				return nil, errors.Wrapf(err, "relabel failed %q", spliti[0])
			}
		}
		if rootProp == "" {
			options = append(options, "rprivate")
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

//GetTmpfsMounts takes user provided input for tmpfs mounts and creates Mount structs
func (c *createConfig) GetTmpfsMounts() []spec.Mount {
	var m []spec.Mount
	for _, i := range c.tmpfs {
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

	// Uncomment after talking to mheon about unimplemented funcs
	// options = append(options, libpod.WithLabels(c.labels))

	if c.interactive {
		options = append(options, libpod.WithStdin())
	}
	if c.name != "" {
		logrus.Debug("appending name %s", c.name)
		options = append(options, libpod.WithName(c.name))
	}

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
