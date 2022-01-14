package specgenutil

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/libpod/define"
	ann "github.com/containers/podman/v3/pkg/annotations"
	"github.com/containers/podman/v3/pkg/domain/entities"
	envLib "github.com/containers/podman/v3/pkg/env"
	"github.com/containers/podman/v3/pkg/namespaces"
	"github.com/containers/podman/v3/pkg/specgen"
	systemdDefine "github.com/containers/podman/v3/pkg/systemd/define"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func getCPULimits(c *entities.ContainerCreateOptions) *specs.LinuxCPU {
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

func getIOLimits(s *specgen.SpecGenerator, c *entities.ContainerCreateOptions) (*specs.LinuxBlockIO, error) {
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
		if s.WeightDevice, err = parseWeightDevices(c.BlkIOWeightDevice); err != nil {
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

func getMemoryLimits(s *specgen.SpecGenerator, c *entities.ContainerCreateOptions) (*specs.LinuxMemory, error) {
	var err error
	memory := &specs.LinuxMemory{}
	hasLimits := false
	if m := c.Memory; len(m) > 0 {
		ml, err := units.RAMInBytes(m)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory")
		}
		if ml > 0 {
			memory.Limit = &ml
			if c.MemorySwap == "" {
				limit := 2 * ml
				memory.Swap = &(limit)
			}
			hasLimits = true
		}
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
		// only set memory swap if it was set
		// -1 indicates unlimited
		if m != "-1" {
			ms, err = units.RAMInBytes(m)
			memory.Swap = &ms
			if err != nil {
				return nil, errors.Wrapf(err, "invalid value for memory")
			}
			hasLimits = true
		}
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

func setNamespaces(s *specgen.SpecGenerator, c *entities.ContainerCreateOptions) error {
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
	userns := os.Getenv("PODMAN_USERNS")
	if c.UserNS != "" {
		userns = c.UserNS
	}
	// userns must be treated differently
	if userns != "" {
		s.UserNS, err = specgen.ParseUserNamespace(userns)
		if err != nil {
			return err
		}
	}
	if c.Net != nil {
		s.NetNS = c.Net.Network
	}
	return nil
}

func FillOutSpecGen(s *specgen.SpecGenerator, c *entities.ContainerCreateOptions, args []string) error {
	var (
		err error
	)
	// validate flags as needed
	if err := validate(c); err != nil {
		return err
	}
	s.User = c.User
	var inputCommand []string
	if !c.IsInfra {
		if len(args) > 1 {
			inputCommand = args[1:]
		}
	}

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
	if err := setNamespaces(s, c); err != nil {
		return err
	}
	userNS := namespaces.UsernsMode(s.UserNS.NSMode)
	tempIDMap, err := util.ParseIDMapping(namespaces.UsernsMode(c.UserNS), []string{}, []string{}, "", "")
	if err != nil {
		return err
	}
	s.IDMappings, err = util.ParseIDMapping(userNS, c.UIDMap, c.GIDMap, c.SubUIDName, c.SubGIDName)
	if err != nil {
		return err
	}
	if len(s.IDMappings.GIDMap) == 0 {
		s.IDMappings.AutoUserNsOpts.AdditionalGIDMappings = tempIDMap.AutoUserNsOpts.AdditionalGIDMappings
		if s.UserNS.NSMode == specgen.NamespaceMode("auto") {
			s.IDMappings.AutoUserNs = true
		}
	}
	if len(s.IDMappings.UIDMap) == 0 {
		s.IDMappings.AutoUserNsOpts.AdditionalUIDMappings = tempIDMap.AutoUserNsOpts.AdditionalUIDMappings
		if s.UserNS.NSMode == specgen.NamespaceMode("auto") {
			s.IDMappings.AutoUserNs = true
		}
	}
	if tempIDMap.AutoUserNsOpts.Size != 0 {
		s.IDMappings.AutoUserNsOpts.Size = tempIDMap.AutoUserNsOpts.Size
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
	if c.Net != nil {
		s.PortMappings = c.Net.PublishPorts
	}
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

	expose, err := CreateExpose(c.Expose)
	if err != nil {
		return err
	}
	s.Expose = expose

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

	if systemdUnit, exists := osEnv[systemdDefine.EnvVariable]; exists {
		labels[systemdDefine.EnvVariable] = systemdUnit
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

	if len(c.StorageOpts) > 0 {
		opts := make(map[string]string, len(c.StorageOpts))
		for _, opt := range c.StorageOpts {
			split := strings.SplitN(opt, "=", 2)
			if len(split) != 2 {
				return errors.Errorf("storage-opt must be formatted KEY=VALUE")
			}
			opts[split[0]] = split[1]
		}
		s.StorageOpts = opts
	}
	s.WorkDir = c.Workdir
	if c.Entrypoint != nil {
		entrypoint := []string{}
		// Check if entrypoint specified is json
		if err := json.Unmarshal([]byte(*c.Entrypoint), &entrypoint); err != nil {
			entrypoint = append(entrypoint, *c.Entrypoint)
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

	if c.Net != nil {
		s.Networks = c.Net.Networks
	}

	if c.Net != nil {
		s.HostAdd = c.Net.AddHosts
		s.UseImageResolvConf = c.Net.UseImageResolvConf
		s.DNSServers = c.Net.DNSServers
		s.DNSSearch = c.Net.DNSSearch
		s.DNSOptions = c.Net.DNSOptions
		s.NetworkOptions = c.Net.NetworkOptions
		s.UseImageHosts = c.Net.NoHosts
	}
	s.HostUsers = c.HostUsers
	s.ImageVolumeMode = c.ImageVolume
	if s.ImageVolumeMode == "bind" {
		s.ImageVolumeMode = "anonymous"
	}

	s.Systemd = strings.ToLower(c.Systemd)
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

	s.DependencyContainers = c.Requires

	// TODO
	// outside of specgen and oci though
	// defaults to true, check spec/storage
	// s.readonly = c.ReadOnlyTmpFS
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
			case "apparmor":
				s.ContainerSecurityConfig.ApparmorProfile = con[1]
				s.Annotations[define.InspectAnnotationApparmor] = con[1]
			case "label":
				// TODO selinux opts and label opts are the same thing
				s.ContainerSecurityConfig.SelinuxOpts = append(s.ContainerSecurityConfig.SelinuxOpts, con[1])
				s.Annotations[define.InspectAnnotationLabel] = strings.Join(s.ContainerSecurityConfig.SelinuxOpts, ",label=")
			case "mask":
				s.ContainerSecurityConfig.Mask = append(s.ContainerSecurityConfig.Mask, strings.Split(con[1], ":")...)
			case "proc-opts":
				s.ProcOpts = strings.Split(con[1], ",")
			case "seccomp":
				s.SeccompProfilePath = con[1]
				s.Annotations[define.InspectAnnotationSeccomp] = con[1]
			// this option is for docker compatibility, it is the same as unmask=ALL
			case "systempaths":
				if con[1] == "unconfined" {
					s.ContainerSecurityConfig.Unmask = append(s.ContainerSecurityConfig.Unmask, []string{"ALL"}...)
				} else {
					return fmt.Errorf("invalid systempaths option %q, only `unconfined` is supported", con[1])
				}
			case "unmask":
				s.ContainerSecurityConfig.Unmask = append(s.ContainerSecurityConfig.Unmask, con[1:]...)
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

	for _, rule := range c.DeviceCGroupRule {
		dev, err := parseLinuxResourcesDeviceAccess(rule)
		if err != nil {
			return err
		}
		s.DeviceCGroupRule = append(s.DeviceCGroupRule, dev)
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
				return err
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

	s.Secrets, s.EnvSecrets, err = parseSecrets(c.Secrets)
	if err != nil {
		return err
	}

	if c.Personality != "" {
		s.Personality = &specs.LinuxPersonality{}
		s.Personality.Domain = specs.LinuxPersonalityDomain(c.Personality)
	}

	s.Remove = c.Rm
	s.StopTimeout = &c.StopTimeout
	s.Timeout = c.Timeout
	s.Timezone = c.Timezone
	s.Umask = c.Umask
	s.PidFile = c.PidFile
	s.Volatile = c.Rm
	s.UnsetEnv = c.UnsetEnv
	s.UnsetEnvAll = c.UnsetEnvAll

	// Initcontainers
	s.InitContainerType = c.InitContainerType

	t := true
	s.Passwd = &t
	return nil
}

func makeHealthCheckFromCli(inCmd, interval string, retries uint, timeout, startPeriod string) (*manifest.Schema2HealthConfig, error) {
	cmdArr := []string{}
	isArr := true
	err := json.Unmarshal([]byte(inCmd), &cmdArr) // array unmarshalling
	if err != nil {
		cmdArr = strings.SplitN(inCmd, " ", 2) // default for compat
		isArr = false
	}
	// Every healthcheck requires a command
	if len(cmdArr) == 0 {
		return nil, errors.New("Must define a healthcheck command for all healthchecks")
	}
	concat := ""
	if cmdArr[0] == "CMD" || cmdArr[0] == "none" { // this is for compat, we are already split properly for most compat cases
		cmdArr = strings.Fields(inCmd)
	} else if cmdArr[0] != "CMD-SHELL" { // this is for podman side of things, won't contain the keywords
		if isArr && len(cmdArr) > 1 { // an array of consecutive commands
			cmdArr = append([]string{"CMD"}, cmdArr...)
		} else { // one singular command
			if len(cmdArr) == 1 {
				concat = cmdArr[0]
			} else {
				concat = strings.Join(cmdArr[0:], " ")
			}
			cmdArr = append([]string{"CMD-SHELL"}, concat)
		}
	}

	if cmdArr[0] == "none" { // if specified to remove healtcheck
		cmdArr = []string{"NONE"}
	}

	// healthcheck is by default an array, so we simply pass the user input
	hc := manifest.Schema2HealthConfig{
		Test: cmdArr,
	}

	if interval == "disable" {
		interval = "0"
	}
	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-interval")
	}

	hc.Interval = intervalDuration

	if retries < 1 {
		return nil, errors.New("healthcheck-retries must be greater than 0")
	}
	hc.Retries = int(retries)
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-timeout")
	}
	if timeoutDuration < time.Duration(1) {
		return nil, errors.New("healthcheck-timeout must be at least 1 second")
	}
	hc.Timeout = timeoutDuration

	startPeriodDuration, err := time.ParseDuration(startPeriod)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-start-period")
	}
	if startPeriodDuration < time.Duration(0) {
		return nil, errors.New("healthcheck-start-period must be 0 seconds or greater")
	}
	hc.StartPeriod = startPeriodDuration

	return &hc, nil
}

func parseWeightDevices(weightDevs []string) (map[string]specs.LinuxWeightDevice, error) {
	wd := make(map[string]specs.LinuxWeightDevice)
	for _, val := range weightDevs {
		split := strings.SplitN(val, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("bad format: %s", val)
		}
		if !strings.HasPrefix(split[0], "/dev/") {
			return nil, fmt.Errorf("bad format for device path: %s", val)
		}
		weight, err := strconv.ParseUint(split[1], 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid weight for device: %s", val)
		}
		if weight > 0 && (weight < 10 || weight > 1000) {
			return nil, fmt.Errorf("invalid weight for device: %s", val)
		}
		w := uint16(weight)
		wd[split[0]] = specs.LinuxWeightDevice{
			Weight:     &w,
			LeafWeight: nil,
		}
	}
	return wd, nil
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

func parseSecrets(secrets []string) ([]specgen.Secret, map[string]string, error) {
	secretParseError := errors.New("error parsing secret")
	var mount []specgen.Secret
	envs := make(map[string]string)
	for _, val := range secrets {
		// mount only tells if user has set an option that can only be used with mount secret type
		mountOnly := false
		source := ""
		secretType := ""
		target := ""
		var uid, gid uint32
		// default mode 444 octal = 292 decimal
		var mode uint32 = 292
		split := strings.Split(val, ",")

		// --secret mysecret
		if len(split) == 1 {
			mountSecret := specgen.Secret{
				Source: val,
				Target: target,
				UID:    uid,
				GID:    gid,
				Mode:   mode,
			}
			mount = append(mount, mountSecret)
			continue
		}
		// --secret mysecret,opt=opt
		if !strings.Contains(split[0], "=") {
			source = split[0]
			split = split[1:]
		}

		for _, val := range split {
			kv := strings.SplitN(val, "=", 2)
			if len(kv) < 2 {
				return nil, nil, errors.Wrapf(secretParseError, "option %s must be in form option=value", val)
			}
			switch kv[0] {
			case "source":
				source = kv[1]
			case "type":
				if secretType != "" {
					return nil, nil, errors.Wrap(secretParseError, "cannot set more tha one secret type")
				}
				if kv[1] != "mount" && kv[1] != "env" {
					return nil, nil, errors.Wrapf(secretParseError, "type %s is invalid", kv[1])
				}
				secretType = kv[1]
			case "target":
				target = kv[1]
			case "mode":
				mountOnly = true
				mode64, err := strconv.ParseUint(kv[1], 8, 32)
				if err != nil {
					return nil, nil, errors.Wrapf(secretParseError, "mode %s invalid", kv[1])
				}
				mode = uint32(mode64)
			case "uid", "UID":
				mountOnly = true
				uid64, err := strconv.ParseUint(kv[1], 10, 32)
				if err != nil {
					return nil, nil, errors.Wrapf(secretParseError, "UID %s invalid", kv[1])
				}
				uid = uint32(uid64)
			case "gid", "GID":
				mountOnly = true
				gid64, err := strconv.ParseUint(kv[1], 10, 32)
				if err != nil {
					return nil, nil, errors.Wrapf(secretParseError, "GID %s invalid", kv[1])
				}
				gid = uint32(gid64)

			default:
				return nil, nil, errors.Wrapf(secretParseError, "option %s invalid", val)
			}
		}

		if secretType == "" {
			secretType = "mount"
		}
		if source == "" {
			return nil, nil, errors.Wrapf(secretParseError, "no source found %s", val)
		}
		if secretType == "mount" {
			mountSecret := specgen.Secret{
				Source: source,
				Target: target,
				UID:    uid,
				GID:    gid,
				Mode:   mode,
			}
			mount = append(mount, mountSecret)
		}
		if secretType == "env" {
			if mountOnly {
				return nil, nil, errors.Wrap(secretParseError, "UID, GID, Mode options cannot be set with secret type env")
			}
			if target == "" {
				target = source
			}
			envs[target] = source
		}
	}
	return mount, envs, nil
}

var cgroupDeviceType = map[string]bool{
	"a": true, // all
	"b": true, // block device
	"c": true, // character device
}

var cgroupDeviceAccess = map[string]bool{
	"r": true, //read
	"w": true, //write
	"m": true, //mknod
}

// parseLinuxResourcesDeviceAccess parses the raw string passed with the --device-access-add flag
func parseLinuxResourcesDeviceAccess(device string) (specs.LinuxDeviceCgroup, error) {
	var devType, access string
	var major, minor *int64

	value := strings.Split(device, " ")
	if len(value) != 3 {
		return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid device cgroup rule requires type, major:Minor, and access rules: %q", device)
	}

	devType = value[0]
	if !cgroupDeviceType[devType] {
		return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid device type in device-access-add: %s", devType)
	}

	number := strings.SplitN(value[1], ":", 2)
	i, err := strconv.ParseInt(number[0], 10, 64)
	if err != nil {
		return specs.LinuxDeviceCgroup{}, err
	}
	major = &i
	if len(number) == 2 && number[1] != "*" {
		i, err := strconv.ParseInt(number[1], 10, 64)
		if err != nil {
			return specs.LinuxDeviceCgroup{}, err
		}
		minor = &i
	}
	access = value[2]
	for _, c := range strings.Split(access, "") {
		if !cgroupDeviceAccess[c] {
			return specs.LinuxDeviceCgroup{}, fmt.Errorf("invalid device access in device-access-add: %s", c)
		}
	}
	return specs.LinuxDeviceCgroup{
		Allow:  true,
		Type:   devType,
		Major:  major,
		Minor:  minor,
		Access: access,
	}, nil
}
