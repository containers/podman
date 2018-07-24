package createconfig

import (
	"strings"

	"github.com/docker/docker/daemon/caps"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/go-units"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/pkg/rootless"
	"github.com/sirupsen/logrus"
)

const cpuPeriod = 100000

// CreateConfigToOCISpec parses information needed to create a container into an OCI runtime spec
func CreateConfigToOCISpec(config *CreateConfig) (*spec.Spec, error) { //nolint
	cgroupPerm := "ro"
	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}
	g.HostSpecific = true
	addCgroup := true
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
	} else if !config.UsernsMode.IsHost() && config.NetMode.IsHost() {
		addCgroup = false
		g.RemoveMount("/sys")
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        "bind",
			Source:      "/sys",
			Options:     []string{"nosuid", "noexec", "nodev", "ro", "rbind"},
		}
		g.AddMount(sysMnt)
	}
	if rootless.IsRootless() {
		g.RemoveMount("/dev/pts")
		devPts := spec.Mount{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		}
		g.AddMount(devPts)
	}

	if addCgroup {
		cgroupMnt := spec.Mount{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"nosuid", "noexec", "nodev", "relatime", cgroupPerm},
		}
		g.AddMount(cgroupMnt)
	}
	g.SetProcessCwd(config.WorkDir)
	g.SetProcessArgs(config.Command)
	g.SetProcessTerminal(config.Tty)

	for key, val := range config.Annotations {
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
	g.AddProcessEnv("container", "podman")

	canAddResources := !rootless.IsRootless()

	if canAddResources {
		// RESOURCES - MEMORY
		if config.Resources.Memory != 0 {
			g.SetLinuxResourcesMemoryLimit(config.Resources.Memory)
			// If a swap limit is not explicitly set, also set a swap limit
			// Default to double the memory limit
			if config.Resources.MemorySwap == 0 {
				g.SetLinuxResourcesMemorySwap(2 * config.Resources.Memory)
			}
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
		if config.Privileged {
			// If privileged, we need to add all the host devices to the
			// spec.  We do not add the user provided ones because we are
			// already adding them all.
			if err := config.AddPrivilegedDevices(&g); err != nil {
				return nil, err
			}
		} else {
			for _, device := range config.Devices {
				if err := addDevice(&g, device); err != nil {
					return nil, err
				}
			}
		}
	}

	for _, uidmap := range config.IDMappings.UIDMap {
		g.AddLinuxUIDMapping(uint32(uidmap.HostID), uint32(uidmap.ContainerID), uint32(uidmap.Size))
	}
	for _, gidmap := range config.IDMappings.GIDMap {
		g.AddLinuxGIDMapping(uint32(gidmap.HostID), uint32(gidmap.ContainerID), uint32(gidmap.Size))
	}
	// SECURITY OPTS
	g.SetProcessNoNewPrivileges(config.NoNewPrivs)
	g.SetProcessApparmorProfile(config.ApparmorProfile)
	g.SetProcessSelinuxLabel(config.ProcessLabel)
	g.SetLinuxMountLabel(config.MountLabel)

	if canAddResources {
		blockAccessToKernelFilesystems(config, &g)

		// RESOURCES - PIDS
		if config.Resources.PidsLimit != 0 {
			g.SetLinuxResourcesPidsLimit(config.Resources.PidsLimit)
		}
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

	if err := addUserNS(config, &g); err != nil {
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
	configSpec := g.Config

	// HANDLE CAPABILITIES
	// NOTE: Must happen before SECCOMP
	if !config.Privileged {
		if err := setupCapabilities(config, configSpec); err != nil {
			return nil, err
		}
	} else {
		g.SetupPrivileged(true)
	}

	// HANDLE SECCOMP

	if config.SeccompProfilePath != "unconfined" {
		seccompConfig, err := getSeccompConfig(config, configSpec)
		if err != nil {
			return nil, err
		}
		configSpec.Linux.Seccomp = seccompConfig
	}

	// Clear default Seccomp profile from Generator for privileged containers
	if config.SeccompProfilePath == "unconfined" || config.Privileged {
		configSpec.Linux.Seccomp = nil
	}

	// BIND MOUNTS
	if err := config.GetVolumesFrom(); err != nil {
		return nil, errors.Wrap(err, "error getting volume mounts from --volumes-from flag")
	}
	mounts, err := config.GetVolumeMounts(configSpec.Mounts)
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

	if canAddResources {
		// BLOCK IO
		blkio, err := config.CreateBlockIO()
		if err != nil {
			return nil, errors.Wrapf(err, "error creating block io")
		}
		if blkio != nil {
			configSpec.Linux.Resources.BlockIO = blkio
		}
	}

	// If we cannot add resources be sure everything is cleared out
	if !canAddResources {
		configSpec.Linux.Resources = &spec.LinuxResources{}
	}
	return configSpec, nil
}

func blockAccessToKernelFilesystems(config *CreateConfig, g *generate.Generator) {
	if !config.Privileged {
		for _, mp := range []string{
			"/proc/acpi",
			"/proc/kcore",
			"/proc/keys",
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

func addPidNS(config *CreateConfig, g *generate.Generator) error {
	pidMode := config.PidMode
	if pidMode.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.PIDNamespace))
	}
	if pidMode.IsContainer() {
		logrus.Debug("using container pidmode")
	}
	return nil
}

func addUserNS(config *CreateConfig, g *generate.Generator) error {
	if (len(config.IDMappings.UIDMap) > 0 || len(config.IDMappings.GIDMap) > 0) && !config.UsernsMode.IsHost() {
		g.AddOrReplaceLinuxNamespace(spec.UserNamespace, "")
	}
	return nil
}

func addNetNS(config *CreateConfig, g *generate.Generator) error {
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
		return nil
	} else if netMode.IsNS() {
		logrus.Debug("Using ns netmode")
		return nil
	} else if netMode.IsUserDefined() {
		logrus.Debug("Using user defined netmode")
		return nil
	}
	return errors.Errorf("unknown network mode")
}

func addUTSNS(config *CreateConfig, g *generate.Generator) error {
	utsMode := config.UtsMode
	if utsMode.IsHost() {
		return g.RemoveLinuxNamespace(spec.UTSNamespace)
	}
	return nil
}

func addIpcNS(config *CreateConfig, g *generate.Generator) error {
	ipcMode := config.IpcMode
	if ipcMode.IsHost() {
		return g.RemoveLinuxNamespace(spec.IPCNamespace)
	}
	if ipcMode.IsContainer() {
		logrus.Debug("Using container ipcmode")
	}

	return nil
}

func addRlimits(config *CreateConfig, g *generate.Generator) error {
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

func setupCapabilities(config *CreateConfig, configSpec *spec.Spec) error {
	useNotRoot := func(user string) bool {
		if user == "" || user == "root" || user == "0" {
			return false
		}
		return true
	}

	var err error
	var caplist []string
	bounding := configSpec.Process.Capabilities.Bounding
	if useNotRoot(config.User) {
		configSpec.Process.Capabilities.Bounding = caplist
	}
	caplist, err = caps.TweakCapabilities(configSpec.Process.Capabilities.Bounding, config.CapAdd, config.CapDrop)
	if err != nil {
		return err
	}

	configSpec.Process.Capabilities.Bounding = caplist
	configSpec.Process.Capabilities.Permitted = caplist
	configSpec.Process.Capabilities.Inheritable = caplist
	configSpec.Process.Capabilities.Effective = caplist
	configSpec.Process.Capabilities.Ambient = caplist
	if useNotRoot(config.User) {
		caplist, err = caps.TweakCapabilities(bounding, config.CapAdd, config.CapDrop)
		if err != nil {
			return err
		}
	}
	configSpec.Process.Capabilities.Bounding = caplist
	return nil
}
