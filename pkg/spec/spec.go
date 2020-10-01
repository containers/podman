package createconfig

import (
	"strings"

	"github.com/containers/common/pkg/capabilities"
	cconfig "github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/sysinfo"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/cgroups"
	"github.com/containers/podman/v2/pkg/env"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/docker/go-units"
	"github.com/opencontainers/runc/libcontainer/user"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const CpuPeriod = 100000

func GetAvailableGids() (int64, error) {
	idMap, err := user.ParseIDMapFile("/proc/self/gid_map")
	if err != nil {
		return 0, err
	}
	count := int64(0)
	for _, r := range idMap {
		count += r.Count
	}
	return count, nil
}

// CreateConfigToOCISpec parses information needed to create a container into an OCI runtime spec
func (config *CreateConfig) createConfigToOCISpec(runtime *libpod.Runtime, userMounts []spec.Mount) (*spec.Spec, error) {
	cgroupPerm := "ro"
	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}
	// Remove the default /dev/shm mount to ensure we overwrite it
	g.RemoveMount("/dev/shm")
	g.HostSpecific = true
	addCgroup := true
	canMountSys := true

	isRootless := rootless.IsRootless()
	inUserNS := config.User.InNS(isRootless)

	if inUserNS && config.Network.NetMode.IsHost() {
		canMountSys = false
	}

	if config.Security.Privileged && canMountSys {
		cgroupPerm = "rw"
		g.RemoveMount("/sys")
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", "rw"},
		}
		g.AddMount(sysMnt)
	} else if !canMountSys {
		addCgroup = false
		g.RemoveMount("/sys")
		r := "ro"
		if config.Security.Privileged {
			r = "rw"
		}
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        TypeBind,
			Source:      "/sys",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", r, "rbind"},
		}
		g.AddMount(sysMnt)
		if !config.Security.Privileged && isRootless {
			g.AddLinuxMaskedPaths("/sys/kernel")
		}
	}
	var runtimeConfig *cconfig.Config

	if runtime != nil {
		runtimeConfig, err = runtime.GetConfig()
		if err != nil {
			return nil, err
		}
		g.Config.Process.Capabilities.Bounding = runtimeConfig.Containers.DefaultCapabilities
		sysctls, err := util.ValidateSysctls(runtimeConfig.Containers.DefaultSysctls)
		if err != nil {
			return nil, err
		}

		for name, val := range config.Security.Sysctl {
			sysctls[name] = val
		}
		config.Security.Sysctl = sysctls
		if !util.StringInSlice("host", config.Resources.Ulimit) {
			config.Resources.Ulimit = append(runtimeConfig.Containers.DefaultUlimits, config.Resources.Ulimit...)
		}
		if config.Resources.PidsLimit < 0 && !config.cgroupDisabled() {
			config.Resources.PidsLimit = runtimeConfig.Containers.PidsLimit
		}

	} else {
		g.Config.Process.Capabilities.Bounding = cconfig.DefaultCapabilities
		if config.Resources.PidsLimit < 0 && !config.cgroupDisabled() {
			config.Resources.PidsLimit = cconfig.DefaultPidsLimit
		}
	}

	gid5Available := true
	if isRootless {
		nGids, err := GetAvailableGids()
		if err != nil {
			return nil, err
		}
		gid5Available = nGids >= 5
	}
	// When using a different user namespace, check that the GID 5 is mapped inside
	// the container.
	if gid5Available && len(config.User.IDMappings.GIDMap) > 0 {
		mappingFound := false
		for _, r := range config.User.IDMappings.GIDMap {
			if r.ContainerID <= 5 && 5 < r.ContainerID+r.Size {
				mappingFound = true
				break
			}
		}
		if !mappingFound {
			gid5Available = false
		}

	}
	if !gid5Available {
		// If we have no GID mappings, the gid=5 default option would fail, so drop it.
		g.RemoveMount("/dev/pts")
		devPts := spec.Mount{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"rprivate", "nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		}
		g.AddMount(devPts)
	}

	if inUserNS && config.Ipc.IpcMode.IsHost() {
		g.RemoveMount("/dev/mqueue")
		devMqueue := spec.Mount{
			Destination: "/dev/mqueue",
			Type:        TypeBind,
			Source:      "/dev/mqueue",
			Options:     []string{"bind", "nosuid", "noexec", "nodev"},
		}
		g.AddMount(devMqueue)
	}
	if inUserNS && config.Pid.PidMode.IsHost() {
		g.RemoveMount("/proc")
		procMount := spec.Mount{
			Destination: "/proc",
			Type:        TypeBind,
			Source:      "/proc",
			Options:     []string{"rbind", "nosuid", "noexec", "nodev"},
		}
		g.AddMount(procMount)
	}

	if addCgroup {
		cgroupMnt := spec.Mount{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", "relatime", cgroupPerm},
		}
		g.AddMount(cgroupMnt)
	}
	g.SetProcessCwd(config.WorkDir)

	ProcessArgs := make([]string, 0)
	// We need to iterate the input for entrypoint because it is a []string
	// but "" is a legit json input, which translates into a []string with an
	// empty position.  This messes up the eventual command being executed
	// in the container
	for _, a := range config.Entrypoint {
		if len(a) > 0 {
			ProcessArgs = append(ProcessArgs, a)
		}
	}
	// Same issue as explained above for config.Entrypoint.
	for _, a := range config.Command {
		if len(a) > 0 {
			ProcessArgs = append(ProcessArgs, a)
		}
	}

	g.SetProcessArgs(ProcessArgs)
	g.SetProcessTerminal(config.Tty)

	for key, val := range config.Annotations {
		g.AddAnnotation(key, val)
	}

	addedResources := false

	// RESOURCES - MEMORY
	if config.Resources.Memory != 0 {
		g.SetLinuxResourcesMemoryLimit(config.Resources.Memory)
		// If a swap limit is not explicitly set, also set a swap limit
		// Default to double the memory limit
		if config.Resources.MemorySwap == 0 {
			g.SetLinuxResourcesMemorySwap(2 * config.Resources.Memory)
		}
		addedResources = true
	}
	if config.Resources.MemoryReservation != 0 {
		g.SetLinuxResourcesMemoryReservation(config.Resources.MemoryReservation)
		addedResources = true
	}
	if config.Resources.MemorySwap != 0 {
		g.SetLinuxResourcesMemorySwap(config.Resources.MemorySwap)
		addedResources = true
	}
	if config.Resources.KernelMemory != 0 {
		g.SetLinuxResourcesMemoryKernel(config.Resources.KernelMemory)
		addedResources = true
	}
	if config.Resources.MemorySwappiness != -1 {
		g.SetLinuxResourcesMemorySwappiness(uint64(config.Resources.MemorySwappiness))
		addedResources = true
	}
	g.SetLinuxResourcesMemoryDisableOOMKiller(config.Resources.DisableOomKiller)
	g.SetProcessOOMScoreAdj(config.Resources.OomScoreAdj)

	// RESOURCES - CPU
	if config.Resources.CPUShares != 0 {
		g.SetLinuxResourcesCPUShares(config.Resources.CPUShares)
		addedResources = true
	}
	if config.Resources.CPUQuota != 0 {
		g.SetLinuxResourcesCPUQuota(config.Resources.CPUQuota)
		addedResources = true
	}
	if config.Resources.CPUPeriod != 0 {
		g.SetLinuxResourcesCPUPeriod(config.Resources.CPUPeriod)
		addedResources = true
	}
	if config.Resources.CPUs != 0 {
		g.SetLinuxResourcesCPUPeriod(CpuPeriod)
		g.SetLinuxResourcesCPUQuota(int64(config.Resources.CPUs * CpuPeriod))
		addedResources = true
	}
	if config.Resources.CPURtRuntime != 0 {
		g.SetLinuxResourcesCPURealtimeRuntime(config.Resources.CPURtRuntime)
		addedResources = true
	}
	if config.Resources.CPURtPeriod != 0 {
		g.SetLinuxResourcesCPURealtimePeriod(config.Resources.CPURtPeriod)
		addedResources = true
	}
	if config.Resources.CPUsetCPUs != "" {
		g.SetLinuxResourcesCPUCpus(config.Resources.CPUsetCPUs)
		addedResources = true
	}
	if config.Resources.CPUsetMems != "" {
		g.SetLinuxResourcesCPUMems(config.Resources.CPUsetMems)
		addedResources = true
	}

	// Devices
	if config.Security.Privileged {
		// If privileged, we need to add all the host devices to the
		// spec.  We do not add the user provided ones because we are
		// already adding them all.
		if err := AddPrivilegedDevices(&g); err != nil {
			return nil, err
		}
	} else {
		for _, devicePath := range config.Devices {
			if err := DevicesFromPath(&g, devicePath); err != nil {
				return nil, err
			}
		}
		if len(config.Resources.DeviceCgroupRules) != 0 {
			if err := deviceCgroupRules(&g, config.Resources.DeviceCgroupRules); err != nil {
				return nil, err
			}
			addedResources = true
		}
	}

	g.SetProcessNoNewPrivileges(config.Security.NoNewPrivs)

	if !config.Security.Privileged {
		g.SetProcessApparmorProfile(config.Security.ApparmorProfile)
	}

	// Unless already set via the CLI, check if we need to disable process
	// labels or set the defaults.
	if len(config.Security.LabelOpts) == 0 && runtimeConfig != nil {
		if !runtimeConfig.Containers.EnableLabeling {
			// Disabled in the config.
			config.Security.LabelOpts = append(config.Security.LabelOpts, "disable")
		} else if err := config.Security.SetLabelOpts(runtime, &config.Pid, &config.Ipc); err != nil {
			// Defaults!
			return nil, err
		}
	}

	BlockAccessToKernelFilesystems(config.Security.Privileged, config.Pid.PidMode.IsHost(), &g)

	// RESOURCES - PIDS
	if config.Resources.PidsLimit > 0 {
		// if running on rootless on a cgroupv1 machine or using the cgroupfs manager, pids
		// limit is not supported.  If the value is still the default
		// then ignore the settings.  If the caller asked for a
		// non-default, then try to use it.
		setPidLimit := true
		if rootless.IsRootless() {
			cgroup2, err := cgroups.IsCgroup2UnifiedMode()
			if err != nil {
				return nil, err
			}
			if (!cgroup2 || (runtimeConfig != nil && runtimeConfig.Engine.CgroupManager != cconfig.SystemdCgroupsManager)) && config.Resources.PidsLimit == sysinfo.GetDefaultPidsLimit() {
				setPidLimit = false
			}
		}
		if setPidLimit {
			g.SetLinuxResourcesPidsLimit(config.Resources.PidsLimit)
			addedResources = true
		}
	}

	// Make sure to always set the default variables unless overridden in the
	// config.
	var defaultEnv map[string]string
	if runtimeConfig == nil {
		defaultEnv = env.DefaultEnvVariables()
	} else {
		defaultEnv, err = env.ParseSlice(runtimeConfig.Containers.Env)
		if err != nil {
			return nil, errors.Wrap(err, "Env fields in containers.conf failed to parse")
		}
		defaultEnv = env.Join(env.DefaultEnvVariables(), defaultEnv)
	}

	if err := addRlimits(config, &g); err != nil {
		return nil, err
	}

	// NAMESPACES

	if err := config.Pid.ConfigureGenerator(&g); err != nil {
		return nil, err
	}

	if err := config.User.ConfigureGenerator(&g); err != nil {
		return nil, err
	}

	if err := config.Network.ConfigureGenerator(&g); err != nil {
		return nil, err
	}

	if err := config.Uts.ConfigureGenerator(&g, &config.Network, runtime); err != nil {
		return nil, err
	}

	if err := config.Ipc.ConfigureGenerator(&g); err != nil {
		return nil, err
	}

	if err := config.Cgroup.ConfigureGenerator(&g); err != nil {
		return nil, err
	}

	config.Env = env.Join(defaultEnv, config.Env)
	for name, val := range config.Env {
		g.AddProcessEnv(name, val)
	}
	configSpec := g.Config

	// If the container image specifies an label with a
	// capabilities.ContainerImageLabel then split the comma separated list
	// of capabilities and record them.  This list indicates the only
	// capabilities, required to run the container.
	var capRequired []string
	for key, val := range config.Labels {
		if util.StringInSlice(key, capabilities.ContainerImageLabels) {
			capRequired = strings.Split(val, ",")
		}
	}
	config.Security.CapRequired = capRequired

	if err := config.Security.ConfigureGenerator(&g, &config.User); err != nil {
		return nil, err
	}

	// BIND MOUNTS
	configSpec.Mounts = SupercedeUserMounts(userMounts, configSpec.Mounts)
	// Process mounts to ensure correct options
	if err := InitFSMounts(configSpec.Mounts); err != nil {
		return nil, err
	}

	// BLOCK IO
	blkio, err := config.CreateBlockIO()
	if err != nil {
		return nil, errors.Wrapf(err, "error creating block io")
	}
	if blkio != nil {
		configSpec.Linux.Resources.BlockIO = blkio
		addedResources = true
	}

	if rootless.IsRootless() {
		cgroup2, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return nil, err
		}
		if !addedResources {
			configSpec.Linux.Resources = &spec.LinuxResources{}
		}

		canUseResources := cgroup2 && runtimeConfig != nil && (runtimeConfig.Engine.CgroupManager == cconfig.SystemdCgroupsManager)

		if addedResources && !canUseResources {
			return nil, errors.New("invalid configuration, cannot specify resource limits without cgroups v2 and --cgroup-manager=systemd")
		}
		if !canUseResources {
			// Force the resources block to be empty instead of having default values.
			configSpec.Linux.Resources = &spec.LinuxResources{}
		}
	}

	switch config.Cgroup.Cgroups {
	case "disabled":
		if addedResources {
			return nil, errors.New("cannot specify resource limits when cgroups are disabled is specified")
		}
		configSpec.Linux.Resources = &spec.LinuxResources{}
	case "enabled", "no-conmon", "":
		// Do nothing
	default:
		return nil, errors.New("unrecognized option for cgroups; supported are 'default', 'disabled', 'no-conmon'")
	}

	// Add annotations
	if configSpec.Annotations == nil {
		configSpec.Annotations = make(map[string]string)
	}

	if config.CidFile != "" {
		configSpec.Annotations[define.InspectAnnotationCIDFile] = config.CidFile
	}

	if config.Rm {
		configSpec.Annotations[define.InspectAnnotationAutoremove] = define.InspectResponseTrue
	} else {
		configSpec.Annotations[define.InspectAnnotationAutoremove] = define.InspectResponseFalse
	}

	if len(config.VolumesFrom) > 0 {
		configSpec.Annotations[define.InspectAnnotationVolumesFrom] = strings.Join(config.VolumesFrom, ",")
	}

	if config.Security.Privileged {
		configSpec.Annotations[define.InspectAnnotationPrivileged] = define.InspectResponseTrue
	} else {
		configSpec.Annotations[define.InspectAnnotationPrivileged] = define.InspectResponseFalse
	}

	if config.Init {
		configSpec.Annotations[define.InspectAnnotationInit] = define.InspectResponseTrue
	} else {
		configSpec.Annotations[define.InspectAnnotationInit] = define.InspectResponseFalse
	}

	return configSpec, nil
}

func (config *CreateConfig) cgroupDisabled() bool {
	return config.Cgroup.Cgroups == "disabled"
}

func BlockAccessToKernelFilesystems(privileged, pidModeIsHost bool, g *generate.Generator) {
	if !privileged {
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
			"/sys/fs/selinux",
		} {
			g.AddLinuxMaskedPaths(mp)
		}

		if pidModeIsHost && rootless.IsRootless() {
			return
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

func addRlimits(config *CreateConfig, g *generate.Generator) error {
	var (
		isRootless = rootless.IsRootless()
		nofileSet  = false
		nprocSet   = false
	)

	for _, u := range config.Resources.Ulimit {
		if u == "host" {
			if len(config.Resources.Ulimit) != 1 {
				return errors.New("ulimit can use host only once")
			}
			g.Config.Process.Rlimits = nil
			break
		}

		ul, err := units.ParseUlimit(u)
		if err != nil {
			return errors.Wrapf(err, "ulimit option %q requires name=SOFT:HARD, failed to be parsed", u)
		}

		if ul.Name == "nofile" {
			nofileSet = true
		} else if ul.Name == "nproc" {
			nprocSet = true
		}

		g.AddProcessRlimits("RLIMIT_"+strings.ToUpper(ul.Name), uint64(ul.Hard), uint64(ul.Soft))
	}

	// If not explicitly overridden by the user, default number of open
	// files and number of processes to the maximum they can be set to
	// (without overriding a sysctl)
	if !nofileSet {
		max := define.RLimitDefaultValue
		current := define.RLimitDefaultValue
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit); err != nil {
				logrus.Warnf("failed to return RLIMIT_NOFILE ulimit %q", err)
			}
			if rlimit.Cur < current {
				current = rlimit.Cur
			}
			if rlimit.Max < max {
				max = rlimit.Max
			}
		}
		g.AddProcessRlimits("RLIMIT_NOFILE", max, current)
	}
	if !nprocSet {
		max := define.RLimitDefaultValue
		current := define.RLimitDefaultValue
		if isRootless {
			var rlimit unix.Rlimit
			if err := unix.Getrlimit(unix.RLIMIT_NPROC, &rlimit); err != nil {
				logrus.Warnf("failed to return RLIMIT_NPROC ulimit %q", err)
			}
			if rlimit.Cur < current {
				current = rlimit.Cur
			}
			if rlimit.Max < max {
				max = rlimit.Max
			}
		}
		g.AddProcessRlimits("RLIMIT_NPROC", max, current)
	}

	return nil
}
