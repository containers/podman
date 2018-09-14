package createconfig

import (
	"os"
	"path"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/docker/docker/daemon/caps"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/go-units"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
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
	canMountSys := true

	isRootless := rootless.IsRootless()
	inUserNS := isRootless || (len(config.IDMappings.UIDMap) > 0 || len(config.IDMappings.GIDMap) > 0) && !config.UsernsMode.IsHost()

	if inUserNS && config.NetMode.IsHost() {
		canMountSys = false
	}

	if config.Privileged && canMountSys {
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
		if config.Privileged {
			r = "rw"
		}
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        "bind",
			Source:      "/sys",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", r, "rbind"},
		}
		g.AddMount(sysMnt)
	}
	if isRootless {
		g.RemoveMount("/dev/pts")
		devPts := spec.Mount{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"rprivate", "nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		}
		g.AddMount(devPts)
	}
	if inUserNS && config.IpcMode.IsHost() {
		g.RemoveMount("/dev/mqueue")
		devMqueue := spec.Mount{
			Destination: "/dev/mqueue",
			Type:        "bind",
			Source:      "/dev/mqueue",
			Options:     []string{"bind", "nosuid", "noexec", "nodev"},
		}
		g.AddMount(devMqueue)
	}
	if inUserNS && config.PidMode.IsHost() {
		g.RemoveMount("/proc")
		procMount := spec.Mount{
			Destination: "/proc",
			Type:        "bind",
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
	g.SetProcessArgs(config.Command)
	g.SetProcessTerminal(config.Tty)

	for key, val := range config.Annotations {
		g.AddAnnotation(key, val)
	}
	g.SetRootReadonly(config.ReadOnlyRootfs)

	hostname := config.Hostname
	if hostname == "" && (config.NetMode.IsHost() || config.UtsMode.IsHost()) {
		hostname, err = os.Hostname()
		if err != nil {
			return nil, errors.Wrap(err, "unable to retrieve hostname")
		}
	}
	g.RemoveHostname()
	if config.Hostname != "" || !config.UtsMode.IsHost() {
		// Set the hostname in the OCI configuration only
		// if specified by the user or if we are creating
		// a new UTS namespace.
		g.SetHostname(hostname)
	}
	g.AddProcessEnv("HOSTNAME", hostname)

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

	if config.Systemd && (strings.HasSuffix(config.Command[0], "init") ||
		strings.HasSuffix(config.Command[0], "systemd")) {
		if err := setupSystemd(config, &g); err != nil {
			return nil, errors.Wrap(err, "failed to setup systemd")
		}
	}
	for _, i := range config.Tmpfs {
		// Default options if nothing passed
		options := []string{"rw", "rprivate", "noexec", "nosuid", "nodev", "size=65536k"}
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
	if len(mounts) > 0 {
		// If we have overlappings mounts, remove them from the spec in favor of
		// the user-added volume mounts
		destinations := make(map[string]bool)
		for _, mount := range mounts {
			destinations[path.Clean(mount.Destination)] = true
		}

		// Copy all mounts from spec to defaultMounts, except for
		//  - mounts overridden by a user supplied mount;
		//  - all mounts under /dev if a user supplied /dev is present;
		mountDev := destinations["/dev"]
		for _, mount := range configSpec.Mounts {
			if _, ok := destinations[path.Clean(mount.Destination)]; !ok {
				if mountDev && strings.HasPrefix(mount.Destination, "/dev/") {
					// filter out everything under /dev if /dev is user-mounted
					continue
				}

				logrus.Debugf("Adding mount %s", mount.Destination)
				mounts = append(mounts, mount)
			}
		}
		configSpec.Mounts = mounts
	}

	if err := g.SetLinuxRootPropagation("shared"); err != nil {
		return nil, errors.Wrapf(err, "failed to set propagation to rslave")
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

// systemd expects to have /run, /run/lock and /tmp on tmpfs
// It also expects to be able to write to /sys/fs/cgroup/systemd and /var/log/journal

func setupSystemd(config *CreateConfig, g *generate.Generator) error {
	mounts, err := config.GetVolumeMounts([]spec.Mount{})
	if err != nil {
		return err
	}
	options := []string{"rw", "rprivate", "noexec", "nosuid", "nodev"}
	for _, dest := range []string{"/run", "/run/lock", "/sys/fs/cgroup/systemd"} {
		if libpod.MountExists(mounts, dest) {
			continue
		}
		tmpfsMnt := spec.Mount{
			Destination: dest,
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     append(options, "tmpcopyup", "size=65536k"),
		}
		g.AddMount(tmpfsMnt)
	}
	for _, dest := range []string{"/tmp", "/var/log/journal"} {
		if libpod.MountExists(mounts, dest) {
			continue
		}
		tmpfsMnt := spec.Mount{
			Destination: dest,
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     append(options, "tmpcopyup"),
		}
		g.AddMount(tmpfsMnt)
	}
	return nil
}

func addPidNS(config *CreateConfig, g *generate.Generator) error {
	pidMode := config.PidMode
	if IsNS(string(pidMode)) {
		return g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), NS(string(pidMode)))
	}
	if pidMode.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.PIDNamespace))
	}
	if pidMode.IsContainer() {
		logrus.Debug("using container pidmode")
	}
	if IsPod(string(pidMode)) {
		logrus.Debug("using pod pidmode")
	}
	return nil
}

func addUserNS(config *CreateConfig, g *generate.Generator) error {
	if IsNS(string(config.UsernsMode)) {
		g.AddOrReplaceLinuxNamespace(spec.UserNamespace, NS(string(config.UsernsMode)))

		// runc complains if no mapping is specified, even if we join another ns.  So provide a dummy mapping
		g.AddLinuxUIDMapping(uint32(0), uint32(0), uint32(1))
		g.AddLinuxGIDMapping(uint32(0), uint32(0), uint32(1))
	}

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
	} else if IsNS(string(netMode)) {
		logrus.Debug("Using ns netmode")
		return g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, NS(string(netMode)))
	} else if IsPod(string(netMode)) {
		logrus.Debug("Using pod netmode, unless pod is not sharing")
		return nil
	} else if netMode.IsUserDefined() {
		logrus.Debug("Using user defined netmode")
		return nil
	}
	return errors.Errorf("unknown network mode")
}

func addUTSNS(config *CreateConfig, g *generate.Generator) error {
	utsMode := config.UtsMode
	if IsNS(string(utsMode)) {
		return g.AddOrReplaceLinuxNamespace(string(spec.UTSNamespace), NS(string(utsMode)))
	}
	if utsMode.IsHost() {
		return g.RemoveLinuxNamespace(spec.UTSNamespace)
	}
	return nil
}

func addIpcNS(config *CreateConfig, g *generate.Generator) error {
	ipcMode := config.IpcMode
	if IsNS(string(ipcMode)) {
		return g.AddOrReplaceLinuxNamespace(string(spec.IPCNamespace), NS(string(ipcMode)))
	}
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
		kernelMax  uint64 = 1048576
		isRootless        = rootless.IsRootless()
		nofileSet         = false
		nprocSet          = false
	)

	for _, u := range config.Resources.Ulimit {
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
	if !nofileSet && !isRootless {
		g.AddProcessRlimits("RLIMIT_NOFILE", kernelMax, kernelMax)
	}
	if !nprocSet && !isRootless {
		g.AddProcessRlimits("RLIMIT_NPROC", kernelMax, kernelMax)
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
