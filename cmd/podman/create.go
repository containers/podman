package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	ann "github.com/containers/libpod/pkg/annotations"
	"github.com/containers/libpod/pkg/apparmor"
	"github.com/containers/libpod/pkg/inspect"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/containers/libpod/pkg/util"
	libpodVersion "github.com/containers/libpod/version"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	defaultEnvVariables = map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM": "xterm",
	}
)

var createDescription = "Creates a new container from the given image or" +
	" storage and prepares it for running the specified command. The" +
	" container ID is then printed to stdout. You can then start it at" +
	" any time with the podman start <container_id> command. The container" +
	" will be created with the initial state 'created'."

var createCommand = cli.Command{
	Name:                   "create",
	Usage:                  "Create but do not start a container",
	Description:            createDescription,
	Flags:                  createFlags,
	Action:                 createCmd,
	ArgsUsage:              "IMAGE [COMMAND [ARG...]]",
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
}

func createCmd(c *cli.Context) error {
	// TODO should allow user to create based off a directory on the host not just image
	// Need CLI support for this

	if err := validateFlags(c, createFlags); err != nil {
		return err
	}

	if c.String("cidfile") != "" {
		if _, err := os.Stat(c.String("cidfile")); err == nil {
			return errors.Errorf("container id file exists. ensure another container is not using it or delete %s", c.String("cidfile"))
		}
		if err := libpod.WriteFile("", c.String("cidfile")); err != nil {
			return errors.Wrapf(err, "unable to write cidfile %s", c.String("cidfile"))
		}
	}

	if len(c.Args()) < 1 {
		return errors.Errorf("image name or ID is required")
	}

	rootfs := ""
	if c.Bool("rootfs") {
		rootfs = c.Args()[0]
	}

	mappings, err := util.ParseIDMapping(c.StringSlice("uidmap"), c.StringSlice("gidmap"), c.String("subuidmap"), c.String("subgidmap"))
	if err != nil {
		return err
	}
	storageOpts, err := libpodruntime.GetDefaultStoreOptions()
	if err != nil {
		return err
	}
	storageOpts.UIDMap = mappings.UIDMap
	storageOpts.GIDMap = mappings.GIDMap

	runtime, err := libpodruntime.GetRuntimeWithStorageOpts(c, &storageOpts)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	rtc := runtime.GetConfig()
	ctx := getContext()

	imageName := ""
	var data *inspect.ImageData = nil
	if rootfs == "" {
		newImage, err := runtime.ImageRuntime().New(ctx, c.Args()[0], rtc.SignaturePolicyPath, "", os.Stderr, nil, image.SigningOptions{}, false, false)
		if err != nil {
			return err
		}
		data, err = newImage.Inspect(ctx)
		imageName = newImage.Names()[0]
	}
	createConfig, err := parseCreateOpts(ctx, c, runtime, imageName, data)
	if err != nil {
		return err
	}

	runtimeSpec, err := cc.CreateConfigToOCISpec(createConfig)
	if err != nil {
		return err
	}

	options, err := createConfig.GetContainerCreateOptions(runtime)
	if err != nil {
		return err
	}

	ctr, err := runtime.NewContainer(ctx, runtimeSpec, options...)
	if err != nil {
		return err
	}

	createConfigJSON, err := json.Marshal(createConfig)
	if err != nil {
		return err
	}
	if err := ctr.AddArtifact("create-config", createConfigJSON); err != nil {
		return err
	}

	logrus.Debug("new container created ", ctr.ID())

	if c.String("cidfile") != "" {
		err := libpod.WriteFile(ctr.ID(), c.String("cidfile"))
		if err != nil {
			logrus.Error(err)
		}
	}
	fmt.Printf("%s\n", ctr.ID())
	return nil
}

func parseSecurityOpt(config *cc.CreateConfig, securityOpts []string) error {
	var (
		labelOpts []string
		err       error
	)

	if config.PidMode.IsHost() {
		labelOpts = append(labelOpts, label.DisableSecOpt()...)
	} else if config.PidMode.IsContainer() {
		ctr, err := config.Runtime.LookupContainer(config.PidMode.Container())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", config.PidMode.Container())
		}
		labelOpts = append(labelOpts, label.DupSecOpt(ctr.ProcessLabel())...)
	}

	if config.IpcMode.IsHost() {
		labelOpts = append(labelOpts, label.DisableSecOpt()...)
	} else if config.IpcMode.IsContainer() {
		ctr, err := config.Runtime.LookupContainer(config.IpcMode.Container())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", config.IpcMode.Container())
		}
		labelOpts = append(labelOpts, label.DupSecOpt(ctr.ProcessLabel())...)
	}

	for _, opt := range securityOpts {
		if opt == "no-new-privileges" {
			config.NoNewPrivs = true
		} else {
			con := strings.SplitN(opt, "=", 2)
			if len(con) != 2 {
				return fmt.Errorf("Invalid --security-opt 1: %q", opt)
			}

			switch con[0] {
			case "label":
				labelOpts = append(labelOpts, con[1])
			case "apparmor":
				config.ApparmorProfile = con[1]
			case "seccomp":
				config.SeccompProfilePath = con[1]
			default:
				return fmt.Errorf("Invalid --security-opt 2: %q", opt)
			}
		}
	}

	if config.ApparmorProfile == "" && apparmor.IsEnabled() {
		// Unless specified otherwise, make sure that the default AppArmor
		// profile is installed.  To avoid redundantly loading the profile
		// on each invocation, check if it's loaded before installing it.
		// Suffix the profile with the current libpod version to allow
		// loading the new, potentially updated profile after an update.
		profile := fmt.Sprintf("%s-%s", apparmor.DefaultLibpodProfile, libpodVersion.Version)

		loadProfile := func() error {
			isLoaded, err := apparmor.IsLoaded(profile)
			if err != nil {
				return err
			}
			if !isLoaded {
				err = apparmor.InstallDefault(profile)
				if err != nil {
					return err
				}

			}
			return nil
		}

		if err := loadProfile(); err != nil {
			switch err {
			case apparmor.ErrApparmorUnsupported:
				// do not set the profile when AppArmor isn't supported
				logrus.Debugf("AppArmor is not supported: setting empty profile")
			default:
				return err
			}
		} else {
			logrus.Infof("Sucessfully loaded AppAmor profile '%s'", profile)
			config.ApparmorProfile = profile
		}
	} else if config.ApparmorProfile != "" && config.ApparmorProfile != "unconfined" {
		if !apparmor.IsEnabled() {
			return fmt.Errorf("profile specified but AppArmor is disabled on the host")
		}

		isLoaded, err := apparmor.IsLoaded(config.ApparmorProfile)
		if err != nil {
			switch err {
			case apparmor.ErrApparmorUnsupported:
				return fmt.Errorf("profile specified but AppArmor is not supported")
			default:
				return fmt.Errorf("error checking if AppArmor profile is loaded: %v", err)
			}
		}
		if !isLoaded {
			return fmt.Errorf("specified AppArmor profile '%s' is not loaded", config.ApparmorProfile)
		}
	}

	if config.SeccompProfilePath == "" {
		if _, err := os.Stat(libpod.SeccompOverridePath); err == nil {
			config.SeccompProfilePath = libpod.SeccompOverridePath
		} else {
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "can't check if %q exists", libpod.SeccompOverridePath)
			}
			if _, err := os.Stat(libpod.SeccompDefaultPath); err != nil {
				if !os.IsNotExist(err) {
					return errors.Wrapf(err, "can't check if %q exists", libpod.SeccompDefaultPath)
				}
			} else {
				config.SeccompProfilePath = libpod.SeccompDefaultPath
			}
		}
	}
	config.ProcessLabel, config.MountLabel, err = label.InitLabels(labelOpts)
	return err
}

// isPortInPortBindings determines if an exposed host port is in user
// provided ports
func isPortInPortBindings(pb map[nat.Port][]nat.PortBinding, port nat.Port) bool {
	var hostPorts []string
	for _, i := range pb {
		hostPorts = append(hostPorts, i[0].HostPort)
	}
	return util.StringInSlice(port.Port(), hostPorts)
}

// isPortInImagePorts determines if an exposed host port was given to us by metadata
// in the image itself
func isPortInImagePorts(exposedPorts map[string]struct{}, port string) bool {
	for i := range exposedPorts {
		fields := strings.Split(i, "/")
		if port == fields[0] {
			return true
		}
	}
	return false
}

// Parses CLI options related to container creation into a config which can be
// parsed into an OCI runtime spec
func parseCreateOpts(ctx context.Context, c *cli.Context, runtime *libpod.Runtime, imageName string, data *inspect.ImageData) (*cc.CreateConfig, error) {
	var (
		inputCommand, command                                    []string
		memoryLimit, memoryReservation, memorySwap, memoryKernel int64
		blkioWeight                                              uint16
	)
	idmappings, err := util.ParseIDMapping(c.StringSlice("uidmap"), c.StringSlice("gidmap"), c.String("subuidname"), c.String("subgidname"))
	if err != nil {
		return nil, err
	}

	if c.String("mac-address") != "" {
		return nil, errors.Errorf("--mac-address option not currently supported")
	}

	imageID := ""

	inputCommand = c.Args()[1:]
	if data != nil {
		imageID = data.ID
	}

	rootfs := ""
	if c.Bool("rootfs") {
		rootfs = c.Args()[0]
	}

	sysctl, err := validateSysctl(c.StringSlice("sysctl"))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid value for sysctl")
	}

	if c.String("memory") != "" {
		memoryLimit, err = units.RAMInBytes(c.String("memory"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory")
		}
	}
	if c.String("memory-reservation") != "" {
		memoryReservation, err = units.RAMInBytes(c.String("memory-reservation"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory-reservation")
		}
	}
	if c.String("memory-swap") != "" {
		memorySwap, err = units.RAMInBytes(c.String("memory-swap"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory-swap")
		}
	}
	if c.String("kernel-memory") != "" {
		memoryKernel, err = units.RAMInBytes(c.String("kernel-memory"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for kernel-memory")
		}
	}
	if c.String("blkio-weight") != "" {
		u, err := strconv.ParseUint(c.String("blkio-weight"), 10, 16)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for blkio-weight")
		}
		blkioWeight = uint16(u)
	}

	if err = parseVolumes(c.StringSlice("volume")); err != nil {
		return nil, err
	}

	if err = parseVolumesFrom(c.StringSlice("volumes-from")); err != nil {
		return nil, err
	}

	tty := c.Bool("tty")

	if c.Bool("detach") && c.Bool("rm") {
		return nil, errors.Errorf("--rm and --detach can not be specified together")
	}
	if c.Int64("cpu-period") != 0 && c.Float64("cpus") > 0 {
		return nil, errors.Errorf("--cpu-period and --cpus cannot be set together")
	}
	if c.Int64("cpu-quota") != 0 && c.Float64("cpus") > 0 {
		return nil, errors.Errorf("--cpu-quota and --cpus cannot be set together")
	}

	// Kernel Namespaces
	var pod *libpod.Pod
	if c.IsSet("pod") {
		pod, err = runtime.LookupPod(c.String("pod"))
		if err != nil {
			return nil, err
		}
	}

	pidModeStr := c.String("pid")
	if !c.IsSet("pid") && pod != nil && pod.SharesPID() {
		pidModeStr = "pod"
	}
	pidMode := container.PidMode(pidModeStr)
	if !cc.Valid(string(pidMode), pidMode) {
		return nil, errors.Errorf("--pid %q is not valid", c.String("pid"))
	}

	usernsModeStr := c.String("userns")
	if !c.IsSet("userns") && pod != nil && pod.SharesUser() {
		usernsModeStr = "pod"
	}
	usernsMode := container.UsernsMode(usernsModeStr)
	if !cc.Valid(string(usernsMode), usernsMode) {
		return nil, errors.Errorf("--userns %q is not valid", c.String("userns"))
	}

	utsModeStr := c.String("uts")
	if !c.IsSet("uts") && pod != nil && pod.SharesUTS() {
		utsModeStr = "pod"
	}
	utsMode := container.UTSMode(utsModeStr)
	if !cc.Valid(string(utsMode), utsMode) {
		return nil, errors.Errorf("--uts %q is not valid", c.String("uts"))
	}

	ipcModeStr := c.String("ipc")
	if !c.IsSet("ipc") && pod != nil && pod.SharesIPC() {
		ipcModeStr = "pod"
	}
	ipcMode := container.IpcMode(ipcModeStr)
	if !cc.Valid(string(ipcMode), ipcMode) {
		return nil, errors.Errorf("--ipc %q is not valid", ipcMode)
	}
	netModeStr := c.String("net")
	if !c.IsSet("net") && pod != nil && pod.SharesNet() {
		netModeStr = "pod"
	}
	// Make sure if network is set to container namespace, port binding is not also being asked for
	netMode := container.NetworkMode(netModeStr)
	if netMode.IsContainer() || cc.IsPod(netModeStr) {
		if len(c.StringSlice("publish")) > 0 || c.Bool("publish-all") {
			return nil, errors.Errorf("cannot set port bindings on an existing container network namespace")
		}
	}

	shmDir := ""
	if ipcMode.IsHost() {
		shmDir = "/dev/shm"
	} else if ipcMode.IsContainer() {
		ctr, err := runtime.LookupContainer(ipcMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", ipcMode.Container())
		}
		shmDir = ctr.ShmDir()
	}

	// USER
	user := c.String("user")
	if user == "" {
		if data == nil {
			user = "0"
		} else {
			user = data.ContainerConfig.User
		}
	}

	// STOP SIGNAL
	stopSignal := syscall.SIGTERM
	signalString := "SIGTERM"
	if data != nil {
		signalString = data.ContainerConfig.StopSignal
	}
	if c.IsSet("stop-signal") {
		signalString = c.String("stop-signal")
	}
	if signalString != "" {
		stopSignal, err = signal.ParseSignal(signalString)
		if err != nil {
			return nil, err
		}
	}

	// ENVIRONMENT VARIABLES
	env := defaultEnvVariables
	if data != nil {
		for _, e := range data.ContainerConfig.Env {
			split := strings.SplitN(e, "=", 2)
			if len(split) > 1 {
				env[split[0]] = split[1]
			} else {
				env[split[0]] = ""
			}
		}
	}
	if err := readKVStrings(env, c.StringSlice("env-file"), c.StringSlice("env")); err != nil {
		return nil, errors.Wrapf(err, "unable to process environment variables")
	}

	// LABEL VARIABLES
	labels, err := getAllLabels(c.StringSlice("label-file"), c.StringSlice("label"))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to process labels")
	}
	if data != nil {
		for key, val := range data.ContainerConfig.Labels {
			if _, ok := labels[key]; !ok {
				labels[key] = val
			}
		}
	}

	// ANNOTATIONS
	annotations := make(map[string]string)
	// First, add our default annotations
	annotations[ann.ContainerType] = "sandbox"
	annotations[ann.TTY] = "false"
	if tty {
		annotations[ann.TTY] = "true"
	}
	if data != nil {
		// Next, add annotations from the image
		for key, value := range data.Annotations {
			annotations[key] = value
		}
	}
	// Last, add user annotations
	for _, annotation := range c.StringSlice("annotation") {
		splitAnnotation := strings.SplitN(annotation, "=", 2)
		if len(splitAnnotation) < 2 {
			return nil, errors.Errorf("Annotations must be formatted KEY=VALUE")
		}
		annotations[splitAnnotation[0]] = splitAnnotation[1]
	}

	// WORKING DIRECTORY
	workDir := "/"
	if c.IsSet("workdir") || c.IsSet("w") {
		workDir = c.String("workdir")
	} else if data != nil && data.ContainerConfig.WorkingDir != "" {
		workDir = data.ContainerConfig.WorkingDir
	}

	// ENTRYPOINT
	// User input entrypoint takes priority over image entrypoint
	entrypoint := c.StringSlice("entrypoint")
	if len(entrypoint) == 0 && data != nil {
		entrypoint = data.ContainerConfig.Entrypoint
	}
	// if entrypoint=, we need to clear the entrypoint
	if len(entrypoint) == 1 && c.IsSet("entrypoint") && strings.Join(c.StringSlice("entrypoint"), "") == "" {
		entrypoint = []string{}
	}
	// Build the command
	// If we have an entry point, it goes first
	if len(entrypoint) > 0 {
		command = entrypoint
	}
	if len(inputCommand) > 0 {
		// User command overrides data CMD
		command = append(command, inputCommand...)
	} else if data != nil && len(data.ContainerConfig.Cmd) > 0 && !c.IsSet("entrypoint") {
		// If not user command, add CMD
		command = append(command, data.ContainerConfig.Cmd...)
	}

	if len(command) == 0 {
		return nil, errors.Errorf("No command specified on command line or as CMD or ENTRYPOINT in this image")
	}

	// EXPOSED PORTS
	var portBindings map[nat.Port][]nat.PortBinding
	if data != nil {
		portBindings, err = cc.ExposedPorts(c.StringSlice("expose"), c.StringSlice("publish"), c.Bool("publish-all"), data.ContainerConfig.ExposedPorts)
		if err != nil {
			return nil, err
		}
	}

	// SHM Size
	shmSize, err := units.FromHumanSize(c.String("shm-size"))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to translate --shm-size")
	}

	// Verify the additional hosts are in correct format
	for _, host := range c.StringSlice("add-host") {
		if _, err := validateExtraHost(host); err != nil {
			return nil, err
		}
	}

	// Check for . and dns-search domains
	if util.StringInSlice(".", c.StringSlice("dns-search")) && len(c.StringSlice("dns-search")) > 1 {
		return nil, errors.Errorf("cannot pass additional search domains when also specifying '.'")
	}

	// Validate domains are good
	for _, dom := range c.StringSlice("dns-search") {
		if _, err := validateDomain(dom); err != nil {
			return nil, err
		}
	}

	var ImageVolumes map[string]struct{}
	if data != nil {
		ImageVolumes = data.ContainerConfig.Volumes
	}
	var imageVolType = map[string]string{
		"bind":   "",
		"tmpfs":  "",
		"ignore": "",
	}
	if _, ok := imageVolType[c.String("image-volume")]; !ok {
		return nil, errors.Errorf("invalid image-volume type %q. Pick one of bind, tmpfs, or ignore", c.String("image-volume"))
	}

	config := &cc.CreateConfig{
		Runtime:           runtime,
		Annotations:       annotations,
		BuiltinImgVolumes: ImageVolumes,
		ConmonPidFile:     c.String("conmon-pidfile"),
		ImageVolumeType:   c.String("image-volume"),
		CapAdd:            c.StringSlice("cap-add"),
		CapDrop:           c.StringSlice("cap-drop"),
		CgroupParent:      c.String("cgroup-parent"),
		Command:           command,
		Detach:            c.Bool("detach"),
		Devices:           c.StringSlice("device"),
		DNSOpt:            c.StringSlice("dns-opt"),
		DNSSearch:         c.StringSlice("dns-search"),
		DNSServers:        c.StringSlice("dns"),
		Entrypoint:        entrypoint,
		Env:               env,
		//ExposedPorts:   ports,
		GroupAdd:       c.StringSlice("group-add"),
		Hostname:       c.String("hostname"),
		HostAdd:        c.StringSlice("add-host"),
		IDMappings:     idmappings,
		Image:          imageName,
		ImageID:        imageID,
		Interactive:    c.Bool("interactive"),
		IP6Address:     c.String("ipv6"),
		IPAddress:      c.String("ip"),
		Labels:         labels,
		LinkLocalIP:    c.StringSlice("link-local-ip"),
		LogDriver:      c.String("log-driver"),
		LogDriverOpt:   c.StringSlice("log-opt"),
		MacAddress:     c.String("mac-address"),
		Name:           c.String("name"),
		Network:        c.String("network"),
		NetworkAlias:   c.StringSlice("network-alias"),
		IpcMode:        ipcMode,
		NetMode:        netMode,
		UtsMode:        utsMode,
		PidMode:        pidMode,
		Pod:            c.String("pod"),
		Privileged:     c.Bool("privileged"),
		Publish:        c.StringSlice("publish"),
		PublishAll:     c.Bool("publish-all"),
		PortBindings:   portBindings,
		Quiet:          c.Bool("quiet"),
		ReadOnlyRootfs: c.Bool("read-only"),
		Resources: cc.CreateResourceConfig{
			BlkioWeight:       blkioWeight,
			BlkioWeightDevice: c.StringSlice("blkio-weight-device"),
			CPUShares:         c.Uint64("cpu-shares"),
			CPUPeriod:         c.Uint64("cpu-period"),
			CPUsetCPUs:        c.String("cpuset-cpus"),
			CPUsetMems:        c.String("cpuset-mems"),
			CPUQuota:          c.Int64("cpu-quota"),
			CPURtPeriod:       c.Uint64("cpu-rt-period"),
			CPURtRuntime:      c.Int64("cpu-rt-runtime"),
			CPUs:              c.Float64("cpus"),
			DeviceReadBps:     c.StringSlice("device-read-bps"),
			DeviceReadIOps:    c.StringSlice("device-read-iops"),
			DeviceWriteBps:    c.StringSlice("device-write-bps"),
			DeviceWriteIOps:   c.StringSlice("device-write-iops"),
			DisableOomKiller:  c.Bool("oom-kill-disable"),
			ShmSize:           shmSize,
			Memory:            memoryLimit,
			MemoryReservation: memoryReservation,
			MemorySwap:        memorySwap,
			MemorySwappiness:  c.Int("memory-swappiness"),
			KernelMemory:      memoryKernel,
			OomScoreAdj:       c.Int("oom-score-adj"),

			PidsLimit: c.Int64("pids-limit"),
			Ulimit:    c.StringSlice("ulimit"),
		},
		Rm:          c.Bool("rm"),
		ShmDir:      shmDir,
		StopSignal:  stopSignal,
		StopTimeout: c.Uint("stop-timeout"),
		Sysctl:      sysctl,
		Tmpfs:       c.StringSlice("tmpfs"),
		Tty:         tty,
		User:        user,
		UsernsMode:  usernsMode,
		Volumes:     c.StringSlice("volume"),
		WorkDir:     workDir,
		Rootfs:      rootfs,
		VolumesFrom: c.StringSlice("volumes-from"),
	}

	if !config.Privileged {
		if err := parseSecurityOpt(config, c.StringSlice("security-opt")); err != nil {
			return nil, err
		}
	}
	config.SecurityOpts = c.StringSlice("security-opt")
	warnings, err := verifyContainerResources(config, false)
	if err != nil {
		return nil, err
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, warning)
	}
	return config, nil
}
