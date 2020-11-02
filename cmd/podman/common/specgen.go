package common

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/libpod/define"
	ann "github.com/containers/podman/v2/pkg/annotations"
	envLib "github.com/containers/podman/v2/pkg/env"
	ns "github.com/containers/podman/v2/pkg/namespaces"
	"github.com/containers/podman/v2/pkg/specgen"
	systemdGen "github.com/containers/podman/v2/pkg/systemd/generate"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func getCPULimits(c *ContainerCLIOpts) *specs.LinuxCPU {
	cpu := &specs.LinuxCPU{}
	hasLimits := false

	if c.CPUS > 0 {
		period, quota := util.CoresToPeriodAndQuota(c.CPUS)

		cpu.Period = &period
		cpu.Quota = &quota
		hasLimits = true
	}
	if c.CPUShares > 0 {
		cpu.Shares = &c.CPUShares
		hasLimits = true
	}
	if c.CPUPeriod > 0 {
		cpu.Period = &c.CPUPeriod
		hasLimits = true
	}
	if c.CPUSetCPUs != "" {
		cpu.Cpus = c.CPUSetCPUs
		hasLimits = true
	}
	if c.CPUSetMems != "" {
		cpu.Mems = c.CPUSetMems
		hasLimits = true
	}
	if c.CPUQuota > 0 {
		cpu.Quota = &c.CPUQuota
		hasLimits = true
	}
	if c.CPURTPeriod > 0 {
		cpu.RealtimePeriod = &c.CPURTPeriod
		hasLimits = true
	}
	if c.CPURTRuntime > 0 {
		cpu.RealtimeRuntime = &c.CPURTRuntime
		hasLimits = true
	}

	if !hasLimits {
		return nil
	}
	return cpu
}

func getIOLimits(s *specgen.SpecGenerator, c *ContainerCLIOpts) (*specs.LinuxBlockIO, error) {
	var err error
	io := &specs.LinuxBlockIO{}
	hasLimits := false
	if b := c.BlkIOWeight; len(b) > 0 {
		u, err := strconv.ParseUint(b, 10, 16)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for blkio-weight")
		}
		nu := uint16(u)
		io.Weight = &nu
		hasLimits = true
	}

	if len(c.BlkIOWeightDevice) > 0 {
		if err := parseWeightDevices(s, c.BlkIOWeightDevice); err != nil {
			return nil, err
		}
		hasLimits = true
	}

	if bps := c.DeviceReadBPs; len(bps) > 0 {
		if s.ThrottleReadBpsDevice, err = parseThrottleBPSDevices(bps); err != nil {
			return nil, err
		}
		hasLimits = true
	}

	if bps := c.DeviceWriteBPs; len(bps) > 0 {
		if s.ThrottleWriteBpsDevice, err = parseThrottleBPSDevices(bps); err != nil {
			return nil, err
		}
		hasLimits = true
	}

	if iops := c.DeviceReadIOPs; len(iops) > 0 {
		if s.ThrottleReadIOPSDevice, err = parseThrottleIOPsDevices(iops); err != nil {
			return nil, err
		}
		hasLimits = true
	}

	if iops := c.DeviceWriteIOPs; len(iops) > 0 {
		if s.ThrottleWriteIOPSDevice, err = parseThrottleIOPsDevices(iops); err != nil {
			return nil, err
		}
		hasLimits = true
	}

	if !hasLimits {
		return nil, nil
	}
	return io, nil
}

func getMemoryLimits(s *specgen.SpecGenerator, c *ContainerCLIOpts) (*specs.LinuxMemory, error) {
	var err error
	memory := &specs.LinuxMemory{}
	hasLimits := false
	if m := c.Memory; len(m) > 0 {
		ml, err := units.RAMInBytes(m)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory")
		}
		memory.Limit = &ml
		if c.MemorySwap == "" {
			limit := 2 * ml
			memory.Swap = &(limit)
		}
		hasLimits = true
	}
	if m := c.MemoryReservation; len(m) > 0 {
		mr, err := units.RAMInBytes(m)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory")
		}
		memory.Reservation = &mr
		hasLimits = true
	}
	if m := c.MemorySwap; len(m) > 0 {
		var ms int64
		if m == "-1" {
			ms = int64(-1)
			s.ResourceLimits.Memory.Swap = &ms
		} else {
			ms, err = units.RAMInBytes(m)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid value for memory")
			}
		}
		memory.Swap = &ms
		hasLimits = true
	}
	if m := c.KernelMemory; len(m) > 0 {
		mk, err := units.RAMInBytes(m)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for kernel-memory")
		}
		memory.Kernel = &mk
		hasLimits = true
	}
	if c.MemorySwappiness >= 0 {
		swappiness := uint64(c.MemorySwappiness)
		memory.Swappiness = &swappiness
		hasLimits = true
	}
	if c.OOMKillDisable {
		memory.DisableOOMKiller = &c.OOMKillDisable
		hasLimits = true
	}
	if !hasLimits {
		return nil, nil
	}
	return memory, nil
}

func setNamespaces(s *specgen.SpecGenerator, c *ContainerCLIOpts) error {
	var err error

	if c.PID != "" {
		s.PidNS, err = specgen.ParseNamespace(c.PID)
		if err != nil {
			return err
		}
	}
	if c.IPC != "" {
		s.IpcNS, err = specgen.ParseNamespace(c.IPC)
		if err != nil {
			return err
		}
	}
	if c.UTS != "" {
		s.UtsNS, err = specgen.ParseNamespace(c.UTS)
		if err != nil {
			return err
		}
	}
	if c.CgroupNS != "" {
		s.CgroupNS, err = specgen.ParseNamespace(c.CgroupNS)
		if err != nil {
			return err
		}
	}
	// userns must be treated differently
	if c.UserNS != "" {
		s.UserNS, err = specgen.ParseUserNamespace(c.UserNS)
		if err != nil {
			return err
		}
	}
	if c.Net != nil {
		s.NetNS = c.Net.Network
	}
	return nil
}

func FillOutSpecGen(s *specgen.SpecGenerator, c *ContainerCLIOpts, args []string) error {
	var (
		err error
	)

	// validate flags as needed
	if err := c.validate(); err != nil {
		return err
	}

	s.User = c.User
	inputCommand := args[1:]
	if len(c.HealthCmd) > 0 {
		if c.NoHealthCheck {
			return errors.New("Cannot specify both --no-healthcheck and --health-cmd")
		}
		s.HealthConfig, err = makeHealthCheckFromCli(c.HealthCmd, c.HealthInterval, c.HealthRetries, c.HealthTimeout, c.HealthStartPeriod)
		if err != nil {
			return err
		}
	} else if c.NoHealthCheck {
		s.HealthConfig = &manifest.Schema2HealthConfig{
			Test: []string{"NONE"},
		}
	}

	userNS := ns.UsernsMode(c.UserNS)
	s.IDMappings, err = util.ParseIDMapping(userNS, c.UIDMap, c.GIDMap, c.SubUIDName, c.SubGIDName)
	if err != nil {
		return err
	}
	// If some mappings are specified, assume a private user namespace
	if userNS.IsDefaultValue() && (!s.IDMappings.HostUIDMapping || !s.IDMappings.HostGIDMapping) {
		s.UserNS.NSMode = specgen.Private
	} else {
		s.UserNS.NSMode = specgen.NamespaceMode(userNS)
	}

	s.Terminal = c.TTY

	if err := verifyExpose(c.Expose); err != nil {
		return err
	}
	// We are not handling the Expose flag yet.
	// s.PortsExpose = c.Expose
	s.PortMappings = c.Net.PublishPorts
	s.PublishExposedPorts = c.PublishAll
	s.Pod = c.Pod

	if len(c.PodIDFile) > 0 {
		if len(s.Pod) > 0 {
			return errors.New("Cannot specify both --pod and --pod-id-file")
		}
		podID, err := ReadPodIDFile(c.PodIDFile)
		if err != nil {
			return err
		}
		s.Pod = podID
	}

	expose, err := createExpose(c.Expose)
	if err != nil {
		return err
	}
	s.Expose = expose

	if err := setNamespaces(s, c); err != nil {
		return err
	}

	if sig := c.StopSignal; len(sig) > 0 {
		stopSignal, err := util.ParseSignal(sig)
		if err != nil {
			return err
		}
		s.StopSignal = &stopSignal
	}

	// ENVIRONMENT VARIABLES
	//
	// Precedence order (higher index wins):
	//  1) containers.conf (EnvHost, EnvHTTP, Env) 2) image data, 3 User EnvHost/EnvHTTP, 4) env-file, 5) env
	// containers.conf handled and image data handled on the server side
	// user specified EnvHost and EnvHTTP handled on Server Side relative to Server
	// env-file and env handled on client side
	var env map[string]string

	// First transform the os env into a map. We need it for the labels later in
	// any case.
	osEnv, err := envLib.ParseSlice(os.Environ())
	if err != nil {
		return errors.Wrap(err, "error parsing host environment variables")
	}

	s.EnvHost = c.EnvHost
	s.HTTPProxy = c.HTTPProxy

	// env-file overrides any previous variables
	for _, f := range c.EnvFile {
		fileEnv, err := envLib.ParseFile(f)
		if err != nil {
			return err
		}
		// File env is overridden by env.
		env = envLib.Join(env, fileEnv)
	}

	parsedEnv, err := envLib.ParseSlice(c.Env)
	if err != nil {
		return err
	}

	s.Env = envLib.Join(env, parsedEnv)

	// LABEL VARIABLES
	labels, err := parse.GetAllLabels(c.LabelFile, c.Label)
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}

	if systemdUnit, exists := osEnv[systemdGen.EnvVariable]; exists {
		labels[systemdGen.EnvVariable] = systemdUnit
	}

	s.Labels = labels

	// ANNOTATIONS
	annotations := make(map[string]string)

	// First, add our default annotations
	annotations[ann.TTY] = "false"
	if c.TTY {
		annotations[ann.TTY] = "true"
	}

	// Last, add user annotations
	for _, annotation := range c.Annotation {
		splitAnnotation := strings.SplitN(annotation, "=", 2)
		if len(splitAnnotation) < 2 {
			return errors.Errorf("Annotations must be formatted KEY=VALUE")
		}
		annotations[splitAnnotation[0]] = splitAnnotation[1]
	}
	s.Annotations = annotations

	s.WorkDir = c.Workdir
	if c.Entrypoint != nil {
		entrypoint := []string{}
		if ep := *c.Entrypoint; len(ep) > 0 {
			// Check if entrypoint specified is json
			if err := json.Unmarshal([]byte(*c.Entrypoint), &entrypoint); err != nil {
				entrypoint = append(entrypoint, ep)
			}
		}
		s.Entrypoint = entrypoint
	}

	// Include the command used to create the container.
	s.ContainerCreateCommand = os.Args

	if len(inputCommand) > 0 {
		s.Command = inputCommand
	}

	// SHM Size
	if c.ShmSize != "" {
		shmSize, err := units.FromHumanSize(c.ShmSize)
		if err != nil {
			return errors.Wrapf(err, "unable to translate --shm-size")
		}
		s.ShmSize = &shmSize
	}
	s.CNINetworks = c.Net.CNINetworks
	s.HostAdd = c.Net.AddHosts
	s.UseImageResolvConf = c.Net.UseImageResolvConf
	s.DNSServers = c.Net.DNSServers
	s.DNSSearch = c.Net.DNSSearch
	s.DNSOptions = c.Net.DNSOptions
	s.StaticIP = c.Net.StaticIP
	s.StaticMAC = c.Net.StaticMAC
	s.NetworkOptions = c.Net.NetworkOptions
	s.UseImageHosts = c.Net.NoHosts

	s.ImageVolumeMode = c.ImageVolume
	if s.ImageVolumeMode == "bind" {
		s.ImageVolumeMode = "anonymous"
	}

	s.Systemd = c.Systemd
	s.SdNotifyMode = c.SdNotifyMode
	if s.ResourceLimits == nil {
		s.ResourceLimits = &specs.LinuxResources{}
	}
	s.ResourceLimits.Memory, err = getMemoryLimits(s, c)
	if err != nil {
		return err
	}
	s.ResourceLimits.BlockIO, err = getIOLimits(s, c)
	if err != nil {
		return err
	}
	if c.PIDsLimit != nil {
		pids := specs.LinuxPids{
			Limit: *c.PIDsLimit,
		}

		s.ResourceLimits.Pids = &pids
	}
	s.ResourceLimits.CPU = getCPULimits(c)

	unifieds := make(map[string]string)
	for _, unified := range c.CgroupConf {
		splitUnified := strings.SplitN(unified, "=", 2)
		if len(splitUnified) < 2 {
			return errors.Errorf("--cgroup-conf must be formatted KEY=VALUE")
		}
		unifieds[splitUnified[0]] = splitUnified[1]
	}
	if len(unifieds) > 0 {
		s.ResourceLimits.Unified = unifieds
	}

	if s.ResourceLimits.CPU == nil && s.ResourceLimits.Pids == nil && s.ResourceLimits.BlockIO == nil && s.ResourceLimits.Memory == nil && s.ResourceLimits.Unified == nil {
		s.ResourceLimits = nil
	}

	if s.LogConfiguration == nil {
		s.LogConfiguration = &specgen.LogConfig{}
	}
	s.LogConfiguration.Driver = define.KubernetesLogging
	if ld := c.LogDriver; len(ld) > 0 {
		s.LogConfiguration.Driver = ld
	}
	s.CgroupParent = c.CGroupParent
	s.CgroupsMode = c.CGroupsMode
	s.Groups = c.GroupAdd

	s.Hostname = c.Hostname
	sysctl := map[string]string{}
	if ctl := c.Sysctl; len(ctl) > 0 {
		sysctl, err = util.ValidateSysctls(ctl)
		if err != nil {
			return err
		}
	}
	s.Sysctl = sysctl

	s.CapAdd = c.CapAdd
	s.CapDrop = c.CapDrop
	s.Privileged = c.Privileged
	s.ReadOnlyFilesystem = c.ReadOnly
	s.ConmonPidFile = c.ConmonPIDFile

	// TODO
	// ouitside of specgen and oci though
	// defaults to true, check spec/storage
	// s.readon = c.ReadOnlyTmpFS
	//  TODO convert to map?
	// check if key=value and convert
	sysmap := make(map[string]string)
	for _, ctl := range c.Sysctl {
		splitCtl := strings.SplitN(ctl, "=", 2)
		if len(splitCtl) < 2 {
			return errors.Errorf("invalid sysctl value %q", ctl)
		}
		sysmap[splitCtl[0]] = splitCtl[1]
	}
	s.Sysctl = sysmap

	if c.CIDFile != "" {
		s.Annotations[define.InspectAnnotationCIDFile] = c.CIDFile
	}

	for _, opt := range c.SecurityOpt {
		if opt == "no-new-privileges" {
			s.ContainerSecurityConfig.NoNewPrivileges = true
		} else {
			con := strings.SplitN(opt, "=", 2)
			if len(con) != 2 {
				return fmt.Errorf("invalid --security-opt 1: %q", opt)
			}

			switch con[0] {
			case "proc-opts":
				s.ProcOpts = strings.Split(con[1], ",")
			case "label":
				// TODO selinux opts and label opts are the same thing
				s.ContainerSecurityConfig.SelinuxOpts = append(s.ContainerSecurityConfig.SelinuxOpts, con[1])
				s.Annotations[define.InspectAnnotationLabel] = strings.Join(s.ContainerSecurityConfig.SelinuxOpts, ",label=")
			case "apparmor":
				s.ContainerSecurityConfig.ApparmorProfile = con[1]
				s.Annotations[define.InspectAnnotationApparmor] = con[1]
			case "seccomp":
				s.SeccompProfilePath = con[1]
				s.Annotations[define.InspectAnnotationSeccomp] = con[1]
			default:
				return fmt.Errorf("invalid --security-opt 2: %q", opt)
			}
		}
	}

	s.SeccompPolicy = c.SeccompPolicy

	s.VolumesFrom = c.VolumesFrom

	// Only add read-only tmpfs mounts in case that we are read-only and the
	// read-only tmpfs flag has been set.
	mounts, volumes, overlayVolumes, imageVolumes, err := parseVolumes(c.Volume, c.Mount, c.TmpFS, c.ReadOnlyTmpFS && c.ReadOnly)
	if err != nil {
		return err
	}
	s.Mounts = mounts
	s.Volumes = volumes
	s.OverlayVolumes = overlayVolumes
	s.ImageVolumes = imageVolumes

	for _, dev := range c.Devices {
		s.Devices = append(s.Devices, specs.LinuxDevice{Path: dev})
	}

	s.Init = c.Init
	s.InitPath = c.InitPath
	s.Stdin = c.Interactive
	// quiet
	// DeviceCgroupRules: c.StringSlice("device-cgroup-rule"),

	// Rlimits/Ulimits
	for _, u := range c.Ulimit {
		if u == "host" {
			s.Rlimits = nil
			break
		}
		ul, err := units.ParseUlimit(u)
		if err != nil {
			return errors.Wrapf(err, "ulimit option %q requires name=SOFT:HARD, failed to be parsed", u)
		}
		rl := specs.POSIXRlimit{
			Type: ul.Name,
			Hard: uint64(ul.Hard),
			Soft: uint64(ul.Soft),
		}
		s.Rlimits = append(s.Rlimits, rl)
	}

	logOpts := make(map[string]string)
	for _, o := range c.LogOptions {
		split := strings.SplitN(o, "=", 2)
		if len(split) < 2 {
			return errors.Errorf("invalid log option %q", o)
		}
		switch strings.ToLower(split[0]) {
		case "driver":
			s.LogConfiguration.Driver = split[1]
		case "path":
			s.LogConfiguration.Path = split[1]
		case "max-size":
			logSize, err := units.FromHumanSize(split[1])
			if err != nil {
				return errors.Wrapf(err, "%s is not a valid option", o)
			}
			s.LogConfiguration.Size = logSize
		default:
			logOpts[split[0]] = split[1]
		}
	}
	s.LogConfiguration.Options = logOpts
	s.Name = c.Name
	s.PreserveFDs = c.PreserveFDs

	s.OOMScoreAdj = &c.OOMScoreAdj
	if c.Restart != "" {
		splitRestart := strings.Split(c.Restart, ":")
		switch len(splitRestart) {
		case 1:
			// No retries specified
		case 2:
			if strings.ToLower(splitRestart[0]) != "on-failure" {
				return errors.Errorf("restart policy retries can only be specified with on-failure restart policy")
			}
			retries, err := strconv.Atoi(splitRestart[1])
			if err != nil {
				return errors.Wrapf(err, "error parsing restart policy retry count")
			}
			if retries < 0 {
				return errors.Errorf("must specify restart policy retry count as a number greater than 0")
			}
			var retriesUint = uint(retries)
			s.RestartRetries = &retriesUint
		default:
			return errors.Errorf("invalid restart policy: may specify retries at most once")
		}
		s.RestartPolicy = splitRestart[0]
	}
	s.Remove = c.Rm
	s.StopTimeout = &c.StopTimeout
	s.Timezone = c.Timezone
	s.Umask = c.Umask

	return nil
}

func makeHealthCheckFromCli(inCmd, interval string, retries uint, timeout, startPeriod string) (*manifest.Schema2HealthConfig, error) {
	// Every healthcheck requires a command
	if len(inCmd) == 0 {
		return nil, errors.New("Must define a healthcheck command for all healthchecks")
	}

	// first try to parse option value as JSON array of strings...
	cmd := []string{}

	if inCmd == "none" {
		cmd = []string{"NONE"}
	} else {
		err := json.Unmarshal([]byte(inCmd), &cmd)
		if err != nil {
			// ...otherwise pass it to "/bin/sh -c" inside the container
			cmd = []string{"CMD-SHELL", inCmd}
		}
	}
	hc := manifest.Schema2HealthConfig{
		Test: cmd,
	}

	if interval == "disable" {
		interval = "0"
	}
	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-interval %s ", interval)
	}

	hc.Interval = intervalDuration

	if retries < 1 {
		return nil, errors.New("healthcheck-retries must be greater than 0")
	}
	hc.Retries = int(retries)
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-timeout %s", timeout)
	}
	if timeoutDuration < time.Duration(1) {
		return nil, errors.New("healthcheck-timeout must be at least 1 second")
	}
	hc.Timeout = timeoutDuration

	startPeriodDuration, err := time.ParseDuration(startPeriod)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-start-period %s", startPeriod)
	}
	if startPeriodDuration < time.Duration(0) {
		return nil, errors.New("healthcheck-start-period must be 0 seconds or greater")
	}
	hc.StartPeriod = startPeriodDuration

	return &hc, nil
}

func parseWeightDevices(s *specgen.SpecGenerator, weightDevs []string) error {
	for _, val := range weightDevs {
		split := strings.SplitN(val, ":", 2)
		if len(split) != 2 {
			return fmt.Errorf("bad format: %s", val)
		}
		if !strings.HasPrefix(split[0], "/dev/") {
			return fmt.Errorf("bad format for device path: %s", val)
		}
		weight, err := strconv.ParseUint(split[1], 10, 0)
		if err != nil {
			return fmt.Errorf("invalid weight for device: %s", val)
		}
		if weight > 0 && (weight < 10 || weight > 1000) {
			return fmt.Errorf("invalid weight for device: %s", val)
		}
		w := uint16(weight)
		s.WeightDevice[split[0]] = specs.LinuxWeightDevice{
			Weight:     &w,
			LeafWeight: nil,
		}
	}
	return nil
}

func parseThrottleBPSDevices(bpsDevices []string) (map[string]specs.LinuxThrottleDevice, error) {
	td := make(map[string]specs.LinuxThrottleDevice)
	for _, val := range bpsDevices {
		split := strings.SplitN(val, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("bad format: %s", val)
		}
		if !strings.HasPrefix(split[0], "/dev/") {
			return nil, fmt.Errorf("bad format for device path: %s", val)
		}
		rate, err := units.RAMInBytes(split[1])
		if err != nil {
			return nil, fmt.Errorf("invalid rate for device: %s. The correct format is <device-path>:<number>[<unit>]. Number must be a positive integer. Unit is optional and can be kb, mb, or gb", val)
		}
		if rate < 0 {
			return nil, fmt.Errorf("invalid rate for device: %s. The correct format is <device-path>:<number>[<unit>]. Number must be a positive integer. Unit is optional and can be kb, mb, or gb", val)
		}
		td[split[0]] = specs.LinuxThrottleDevice{Rate: uint64(rate)}
	}
	return td, nil
}

func parseThrottleIOPsDevices(iopsDevices []string) (map[string]specs.LinuxThrottleDevice, error) {
	td := make(map[string]specs.LinuxThrottleDevice)
	for _, val := range iopsDevices {
		split := strings.SplitN(val, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("bad format: %s", val)
		}
		if !strings.HasPrefix(split[0], "/dev/") {
			return nil, fmt.Errorf("bad format for device path: %s", val)
		}
		rate, err := strconv.ParseUint(split[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid rate for device: %s. The correct format is <device-path>:<number>. Number must be a positive integer", val)
		}
		td[split[0]] = specs.LinuxThrottleDevice{Rate: rate}
	}
	return td, nil
}
