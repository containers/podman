package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/libpod"
	ann "github.com/containers/libpod/pkg/annotations"
	envLib "github.com/containers/libpod/pkg/env"
	ns "github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/libpod/pkg/specgen"
	systemdGen "github.com/containers/libpod/pkg/systemd/generate"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func FillOutSpecGen(s *specgen.SpecGenerator, c *ContainerCLIOpts, args []string) error {
	var (
		err error
		//namespaces map[string]string
	)

	// validate flags as needed
	if err := c.validate(); err != nil {
		return nil
	}

	inputCommand := args[1:]
	if len(c.HealthCmd) > 0 {
		s.HealthConfig, err = makeHealthCheckFromCli(c.HealthCmd, c.HealthInterval, c.HealthRetries, c.HealthTimeout, c.HealthStartPeriod)
		if err != nil {
			return err
		}
	}

	s.IDMappings, err = util.ParseIDMapping(ns.UsernsMode(c.UserNS), c.UIDMap, c.GIDMap, c.SubUIDName, c.SubGIDName)
	if err != nil {
		return err
	}
	if m := c.Memory; len(m) > 0 {
		ml, err := units.RAMInBytes(m)
		if err != nil {
			return errors.Wrapf(err, "invalid value for memory")
		}
		s.ResourceLimits.Memory.Limit = &ml
	}
	if m := c.MemoryReservation; len(m) > 0 {
		mr, err := units.RAMInBytes(m)
		if err != nil {
			return errors.Wrapf(err, "invalid value for memory")
		}
		s.ResourceLimits.Memory.Reservation = &mr
	}
	if m := c.MemorySwap; len(m) > 0 {
		var ms int64
		if m == "-1" {
			ms = int64(-1)
			s.ResourceLimits.Memory.Swap = &ms
		} else {
			ms, err = units.RAMInBytes(m)
			if err != nil {
				return errors.Wrapf(err, "invalid value for memory")
			}
		}
		s.ResourceLimits.Memory.Swap = &ms
	}
	if m := c.KernelMemory; len(m) > 0 {
		mk, err := units.RAMInBytes(m)
		if err != nil {
			return errors.Wrapf(err, "invalid value for kernel-memory")
		}
		s.ResourceLimits.Memory.Kernel = &mk
	}
	if b := c.BlkIOWeight; len(b) > 0 {
		u, err := strconv.ParseUint(b, 10, 16)
		if err != nil {
			return errors.Wrapf(err, "invalid value for blkio-weight")
		}
		nu := uint16(u)
		s.ResourceLimits.BlockIO.Weight = &nu
	}

	s.Terminal = c.TTY
	ep, err := ExposedPorts(c.Expose, c.Net.PublishPorts, c.PublishAll, nil)
	if err != nil {
		return err
	}
	s.PortMappings = ep
	s.Pod = c.Pod

	//s.CgroupNS = specgen.Namespace{
	//	NSMode: ,
	//	Value:  "",
	//}

	//s.UserNS = specgen.Namespace{}

	// Kernel Namespaces
	// TODO Fix handling of namespace from pod
	// Instead of integrating here, should be done in libpod
	// However, that also involves setting up security opts
	// when the pod's namespace is integrated
	//namespaces = map[string]string{
	//	"cgroup": c.CGroupsNS,
	//	"pid":    c.PID,
	//	//"net":    c.Net.Network.Value,   // TODO need help here
	//	"ipc":  c.IPC,
	//	"user": c.User,
	//	"uts":  c.UTS,
	//}
	//
	//if len(c.PID) > 0 {
	//	split := strings.SplitN(c.PID, ":", 2)
	//	// need a way to do thsi
	//	specgen.Namespace{
	//		NSMode: split[0],
	//	}
	//	//Value:  split1 if len allows
	//}
	// TODO this is going to have be done after things like pod creation are done because
	// pod creation changes these values.
	//pidMode := ns.PidMode(namespaces["pid"])
	//usernsMode := ns.UsernsMode(namespaces["user"])
	//utsMode := ns.UTSMode(namespaces["uts"])
	//cgroupMode := ns.CgroupMode(namespaces["cgroup"])
	//ipcMode := ns.IpcMode(namespaces["ipc"])
	//// Make sure if network is set to container namespace, port binding is not also being asked for
	//netMode := ns.NetworkMode(namespaces["net"])
	//if netMode.IsContainer() {
	//	if len(portBindings) > 0 {
	//		return nil, errors.Errorf("cannot set port bindings on an existing container network namespace")
	//	}
	//}

	// TODO Remove when done with namespaces for realz
	// Setting a default for IPC to get this working
	s.IpcNS = specgen.Namespace{
		NSMode: specgen.Private,
		Value:  "",
	}

	// TODO this is going to have to be done the libpod/server end of things
	// USER
	//user := c.String("user")
	//if user == "" {
	//	switch {
	//	case usernsMode.IsKeepID():
	//		user = fmt.Sprintf("%d:%d", rootless.GetRootlessUID(), rootless.GetRootlessGID())
	//	case data == nil:
	//		user = "0"
	//	default:
	//		user = data.Config.User
	//	}
	//}

	// STOP SIGNAL
	signalString := "TERM"
	if sig := c.StopSignal; len(sig) > 0 {
		signalString = sig
	}
	stopSignal, err := util.ParseSignal(signalString)
	if err != nil {
		return err
	}
	s.StopSignal = &stopSignal

	// ENVIRONMENT VARIABLES
	//
	// Precedence order (higher index wins):
	//  1) env-host, 2) image data, 3) env-file, 4) env
	env := map[string]string{
		"container": "podman",
	}

	// First transform the os env into a map. We need it for the labels later in
	// any case.
	osEnv, err := envLib.ParseSlice(os.Environ())
	if err != nil {
		return errors.Wrap(err, "error parsing host environment variables")
	}

	if c.EnvHost {
		env = envLib.Join(env, osEnv)
	}
	// env-file overrides any previous variables
	for _, f := range c.EnvFile {
		fileEnv, err := envLib.ParseFile(f)
		if err != nil {
			return err
		}
		// File env is overridden by env.
		env = envLib.Join(env, fileEnv)
	}

	// env overrides any previous variables
	if cmdLineEnv := c.env; len(cmdLineEnv) > 0 {
		parsedEnv, err := envLib.ParseSlice(cmdLineEnv)
		if err != nil {
			return err
		}
		env = envLib.Join(env, parsedEnv)
	}
	s.Env = env

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

	workDir := "/"
	if wd := c.Workdir; len(wd) > 0 {
		workDir = wd
	}
	s.WorkDir = workDir
	entrypoint := []string{}
	userCommand := []string{}
	if ep := c.Entrypoint; len(ep) > 0 {
		// Check if entrypoint specified is json
		if err := json.Unmarshal([]byte(c.Entrypoint), &entrypoint); err != nil {
			entrypoint = append(entrypoint, ep)
		}
	}

	var command []string

	// Build the command
	// If we have an entry point, it goes first
	if len(entrypoint) > 0 {
		command = entrypoint
	}
	if len(inputCommand) > 0 {
		// User command overrides data CMD
		command = append(command, inputCommand...)
		userCommand = append(userCommand, inputCommand...)
	}

	if len(inputCommand) > 0 {
		s.Command = userCommand
	} else {
		s.Command = command
	}

	// SHM Size
	shmSize, err := units.FromHumanSize(c.ShmSize)
	if err != nil {
		return errors.Wrapf(err, "unable to translate --shm-size")
	}
	s.ShmSize = &shmSize
	s.HostAdd = c.Net.AddHosts
	s.DNSServer = c.Net.DNSServers
	s.DNSSearch = c.Net.DNSSearch
	s.DNSOption = c.Net.DNSOptions

	// deferred, must be added on libpod side
	//var ImageVolumes map[string]struct{}
	//if data != nil && c.String("image-volume") != "ignore" {
	//	ImageVolumes = data.Config.Volumes
	//}

	s.ImageVolumeMode = c.ImageVolume
	systemd := c.SystemdD == "always"
	if !systemd && command != nil {
		x, err := strconv.ParseBool(c.SystemdD)
		if err != nil {
			return errors.Wrapf(err, "cannot parse bool %s", c.SystemdD)
		}
		if x && (command[0] == "/usr/sbin/init" || command[0] == "/sbin/init" || (filepath.Base(command[0]) == "systemd")) {
			systemd = true
		}
	}
	if systemd {
		if s.StopSignal == nil {
			stopSignal, err = util.ParseSignal("RTMIN+3")
			if err != nil {
				return errors.Wrapf(err, "error parsing systemd signal")
			}
			s.StopSignal = &stopSignal
		}
	}
	swappiness := uint64(c.MemorySwappiness)
	if s.ResourceLimits == nil {
		s.ResourceLimits = &specs.LinuxResources{}
	}
	if s.ResourceLimits.Memory == nil {
		s.ResourceLimits.Memory = &specs.LinuxMemory{}
	}
	s.ResourceLimits.Memory.Swappiness = &swappiness

	if s.LogConfiguration == nil {
		s.LogConfiguration = &specgen.LogConfig{}
	}
	s.LogConfiguration.Driver = libpod.KubernetesLogging
	if ld := c.LogDriver; len(ld) > 0 {
		s.LogConfiguration.Driver = ld
	}
	if s.ResourceLimits.Pids == nil {
		s.ResourceLimits.Pids = &specs.LinuxPids{}
	}
	s.ResourceLimits.Pids.Limit = c.PIDsLimit
	if c.CGroups == "disabled" && c.PIDsLimit > 0 {
		s.ResourceLimits.Pids.Limit = -1
	}
	// TODO WTF
	//cgroup := &cc.CgroupConfig{
	//	Cgroups:      c.String("cgroups"),
	//	Cgroupns:     c.String("cgroupns"),
	//	CgroupParent: c.String("cgroup-parent"),
	//	CgroupMode:   cgroupMode,
	//}
	//
	//userns := &cc.UserConfig{
	//	GroupAdd:   c.StringSlice("group-add"),
	//	IDMappings: idmappings,
	//	UsernsMode: usernsMode,
	//	User:       user,
	//}
	//
	//uts := &cc.UtsConfig{
	//	UtsMode:  utsMode,
	//	NoHosts:  c.Bool("no-hosts"),
	//	HostAdd:  c.StringSlice("add-host"),
	//	Hostname: c.String("hostname"),
	//}

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

	// TODO
	// ouitside of specgen and oci though
	// defaults to true, check spec/storage
	//s.readon = c.ReadOnlyTmpFS
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

	for _, opt := range c.SecurityOpt {
		if opt == "no-new-privileges" {
			s.ContainerSecurityConfig.NoNewPrivileges = true
		} else {
			con := strings.SplitN(opt, "=", 2)
			if len(con) != 2 {
				return fmt.Errorf("invalid --security-opt 1: %q", opt)
			}

			switch con[0] {
			case "label":
				// TODO selinux opts and label opts are the same thing
				s.ContainerSecurityConfig.SelinuxOpts = append(s.ContainerSecurityConfig.SelinuxOpts, con[1])
			case "apparmor":
				s.ContainerSecurityConfig.ApparmorProfile = con[1]
			case "seccomp":
				s.SeccompProfilePath = con[1]
			default:
				return fmt.Errorf("invalid --security-opt 2: %q", opt)
			}
		}
	}

	// TODO any idea why this was done
	// storage.go from spec/
	// grab it
	//volumes := rtc.Containers.Volumes
	// TODO conflict on populate?
	//if v := c.Volume; len(v)> 0 {
	//	s.Volumes = append(volumes, c.StringSlice("volume")...)
	//}
	//s.volu

	//s.Mounts = c.Mount
	s.VolumesFrom = c.VolumesFrom

	// TODO any idea why this was done
	//devices := rtc.Containers.Devices
	// TODO conflict on populate?
	//
	//if c.Changed("device") {
	//	devices = append(devices, c.StringSlice("device")...)
	//}

	// TODO things i cannot find in spec
	// we dont think these are in the spec
	// init - initbinary
	// initpath
	s.Stdin = c.Interactive
	// quiet
	//DeviceCgroupRules: c.StringSlice("device-cgroup-rule"),

	if bps := c.DeviceReadBPs; len(bps) > 0 {
		if s.ThrottleReadBpsDevice, err = parseThrottleBPSDevices(bps); err != nil {
			return err
		}
	}

	if bps := c.DeviceWriteBPs; len(bps) > 0 {
		if s.ThrottleWriteBpsDevice, err = parseThrottleBPSDevices(bps); err != nil {
			return err
		}
	}

	if iops := c.DeviceReadIOPs; len(iops) > 0 {
		if s.ThrottleReadIOPSDevice, err = parseThrottleIOPsDevices(iops); err != nil {
			return err
		}
	}

	if iops := c.DeviceWriteIOPs; len(iops) > 0 {
		if s.ThrottleWriteIOPSDevice, err = parseThrottleIOPsDevices(iops); err != nil {
			return err
		}
	}

	s.ResourceLimits.Memory.DisableOOMKiller = &c.OOMKillDisable

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

	//Tmpfs:         c.StringArray("tmpfs"),

	// TODO how to handle this?
	//Syslog:        c.Bool("syslog"),

	logOpts := make(map[string]string)
	for _, o := range c.LogOptions {
		split := strings.SplitN(o, "=", 2)
		if len(split) < 2 {
			return errors.Errorf("invalid log option %q", o)
		}
		logOpts[split[0]] = split[1]
	}
	s.LogConfiguration.Options = logOpts
	s.Name = c.Name

	if err := parseWeightDevices(c.BlkIOWeightDevice, s); err != nil {
		return err
	}

	if s.ResourceLimits.CPU == nil {
		s.ResourceLimits.CPU = &specs.LinuxCPU{}
	}
	s.ResourceLimits.CPU.Shares = &c.CPUShares
	s.ResourceLimits.CPU.Period = &c.CPUPeriod

	// TODO research these
	//s.ResourceLimits.CPU.Cpus = c.CPUS
	//s.ResourceLimits.CPU.Cpus = c.CPUSetCPUs

	//s.ResourceLimits.CPU. = c.CPUSetCPUs
	s.ResourceLimits.CPU.Mems = c.CPUSetMems
	s.ResourceLimits.CPU.Quota = &c.CPUQuota
	s.ResourceLimits.CPU.RealtimePeriod = &c.CPURTPeriod
	s.ResourceLimits.CPU.RealtimeRuntime = &c.CPURTRuntime
	s.OOMScoreAdj = &c.OOMScoreAdj
	s.RestartPolicy = c.Restart
	s.Remove = c.Rm
	s.StopTimeout = &c.StopTimeout

	// TODO where should we do this?
	//func verifyContainerResources(config *cc.CreateConfig, update bool) ([]string, error) {
	return nil
}

func makeHealthCheckFromCli(inCmd, interval string, retries uint, timeout, startPeriod string) (*manifest.Schema2HealthConfig, error) {
	// Every healthcheck requires a command
	if len(inCmd) == 0 {
		return nil, errors.New("Must define a healthcheck command for all healthchecks")
	}

	// first try to parse option value as JSON array of strings...
	cmd := []string{}
	err := json.Unmarshal([]byte(inCmd), &cmd)
	if err != nil {
		// ...otherwise pass it to "/bin/sh -c" inside the container
		cmd = []string{"CMD-SHELL", inCmd}
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
		return nil, errors.New("healthcheck-retries must be greater than 0.")
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

func parseWeightDevices(weightDevs []string, s *specgen.SpecGenerator) error {
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
