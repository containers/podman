package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/cmd/podman/shared/parse"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	ann "github.com/containers/libpod/pkg/annotations"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/inspect"
	ns "github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/libpod/pkg/rootless"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func CreateContainer(ctx context.Context, c *GenericCLIResults, runtime *libpod.Runtime) (*libpod.Container, *cc.CreateConfig, error) {
	var (
		healthCheck *manifest.Schema2HealthConfig
		err         error
		cidFile     *os.File
	)
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(ctx, "createContainer")
		defer span.Finish()
	}
	if c.Bool("rm") && c.String("restart") != "" && c.String("restart") != "no" {
		return nil, nil, errors.Errorf("the --rm option conflicts with --restart")
	}

	rtc, err := runtime.GetConfig()
	if err != nil {
		return nil, nil, err
	}
	rootfs := ""
	if c.Bool("rootfs") {
		rootfs = c.InputArgs[0]
	}

	if c.IsSet("cidfile") {
		cidFile, err = util.OpenExclusiveFile(c.String("cidfile"))
		if err != nil && os.IsExist(err) {
			return nil, nil, errors.Errorf("container id file exists. Ensure another container is not using it or delete %s", c.String("cidfile"))
		}
		if err != nil {
			return nil, nil, errors.Errorf("error opening cidfile %s", c.String("cidfile"))
		}
		defer errorhandling.CloseQuiet(cidFile)
		defer errorhandling.SyncQuiet(cidFile)
	}

	imageName := ""
	var data *inspect.ImageData = nil

	// Set the storage if there is no rootfs specified
	if rootfs == "" {
		var writer io.Writer
		if !c.Bool("quiet") {
			writer = os.Stderr
		}

		name := ""
		if len(c.InputArgs) != 0 {
			name = c.InputArgs[0]
		} else {
			return nil, nil, errors.Errorf("error, image name not provided")
		}

		pullType, err := util.ValidatePullType(c.String("pull"))
		if err != nil {
			return nil, nil, err
		}

		dockerRegistryOptions := image.DockerRegistryOptions{
			OSChoice:           c.String("override-os"),
			ArchitectureChoice: c.String("override-arch"),
		}

		newImage, err := runtime.ImageRuntime().New(ctx, name, rtc.SignaturePolicyPath, c.String("authfile"), writer, &dockerRegistryOptions, image.SigningOptions{}, nil, pullType)
		if err != nil {
			return nil, nil, err
		}
		data, err = newImage.Inspect(ctx)
		if err != nil {
			return nil, nil, err
		}
		names := newImage.Names()
		if len(names) > 0 {
			imageName = names[0]
		} else {
			imageName = newImage.ID()
		}

		// if the user disabled the healthcheck with "none", we skip adding it
		healthCheckCommandInput := c.String("healthcheck-command")

		// the user didnt disable the healthcheck but did pass in a healthcheck command
		// now we need to make a healthcheck from the commandline input
		if healthCheckCommandInput != "none" {
			if len(healthCheckCommandInput) > 0 {
				healthCheck, err = makeHealthCheckFromCli(c)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "unable to create healthcheck")
				}
			} else {
				// the user did not disable the health check and did not pass in a healthcheck
				// command as input.  so now we add healthcheck if it exists AND is correct mediatype
				_, mediaType, err := newImage.Manifest(ctx)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "unable to determine mediatype of image %s", newImage.ID())
				}
				if mediaType == manifest.DockerV2Schema2MediaType {
					healthCheck, err = newImage.GetHealthCheck(ctx)
					if err != nil {
						return nil, nil, errors.Wrapf(err, "unable to get healthcheck for %s", c.InputArgs[0])
					}

					if healthCheck != nil {
						hcCommand := healthCheck.Test
						if len(hcCommand) < 1 || hcCommand[0] == "" || hcCommand[0] == "NONE" {
							// disable health check
							healthCheck = nil
						} else {
							// apply defaults if image doesn't override them
							if healthCheck.Interval == 0 {
								healthCheck.Interval = 30 * time.Second
							}
							if healthCheck.Timeout == 0 {
								healthCheck.Timeout = 30 * time.Second
							}
							/* Docker default is 0s, so the following would be a no-op
							if healthCheck.StartPeriod == 0 {
								healthCheck.StartPeriod = 0 * time.Second
							}
							*/
							if healthCheck.Retries == 0 {
								healthCheck.Retries = 3
							}
						}
					}
				}
			}
		}
	}

	createConfig, err := ParseCreateOpts(ctx, c, runtime, imageName, data)
	if err != nil {
		return nil, nil, err
	}

	// Because parseCreateOpts does derive anything from the image, we add health check
	// at this point. The rest is done by WithOptions.
	createConfig.HealthCheck = healthCheck

	// TODO: Should be able to return this from ParseCreateOpts
	var pod *libpod.Pod
	if createConfig.Pod != "" {
		pod, err = runtime.LookupPod(createConfig.Pod)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error looking up pod to join")
		}
	}

	ctr, err := CreateContainerFromCreateConfig(runtime, createConfig, ctx, pod)
	if err != nil {
		return nil, nil, err
	}
	if cidFile != nil {
		_, err = cidFile.WriteString(ctr.ID())
		if err != nil {
			logrus.Error(err)
		}

	}

	logrus.Debugf("New container created %q", ctr.ID())
	return ctr, createConfig, nil
}

func configureEntrypoint(c *GenericCLIResults, data *inspect.ImageData) []string {
	entrypoint := []string{}
	if c.IsSet("entrypoint") {
		// Force entrypoint to ""
		if c.String("entrypoint") == "" {
			return entrypoint
		}
		// Check if entrypoint specified is json
		if err := json.Unmarshal([]byte(c.String("entrypoint")), &entrypoint); err == nil {
			return entrypoint
		}
		// Return entrypoint as a single command
		return []string{c.String("entrypoint")}
	}
	if data != nil {
		return data.Config.Entrypoint
	}
	return entrypoint
}

func configurePod(c *GenericCLIResults, runtime *libpod.Runtime, namespaces map[string]string, podName string) (map[string]string, error) {
	pod, err := runtime.LookupPod(podName)
	if err != nil {
		return namespaces, err
	}
	podInfraID, err := pod.InfraContainerID()
	if err != nil {
		return namespaces, err
	}
	hasUserns := false
	if podInfraID != "" {
		podCtr, err := runtime.GetContainer(podInfraID)
		if err != nil {
			return namespaces, err
		}
		mappings, err := podCtr.IDMappings()
		if err != nil {
			return namespaces, err
		}
		hasUserns = len(mappings.UIDMap) > 0
	}

	if (namespaces["pid"] == cc.Pod) || (!c.IsSet("pid") && pod.SharesPID()) {
		namespaces["pid"] = fmt.Sprintf("container:%s", podInfraID)
	}
	if (namespaces["net"] == cc.Pod) || (!c.IsSet("net") && !c.IsSet("network") && pod.SharesNet()) {
		namespaces["net"] = fmt.Sprintf("container:%s", podInfraID)
	}
	if hasUserns && (namespaces["user"] == cc.Pod) || (!c.IsSet("user") && pod.SharesUser()) {
		namespaces["user"] = fmt.Sprintf("container:%s", podInfraID)
	}
	if (namespaces["ipc"] == cc.Pod) || (!c.IsSet("ipc") && pod.SharesIPC()) {
		namespaces["ipc"] = fmt.Sprintf("container:%s", podInfraID)
	}
	if (namespaces["uts"] == cc.Pod) || (!c.IsSet("uts") && pod.SharesUTS()) {
		namespaces["uts"] = fmt.Sprintf("container:%s", podInfraID)
	}
	return namespaces, nil
}

// Parses CLI options related to container creation into a config which can be
// parsed into an OCI runtime spec
func ParseCreateOpts(ctx context.Context, c *GenericCLIResults, runtime *libpod.Runtime, imageName string, data *inspect.ImageData) (*cc.CreateConfig, error) {
	var (
		inputCommand, command                                    []string
		memoryLimit, memoryReservation, memorySwap, memoryKernel int64
		blkioWeight                                              uint16
		namespaces                                               map[string]string
	)

	idmappings, err := util.ParseIDMapping(ns.UsernsMode(c.String("userns")), c.StringSlice("uidmap"), c.StringSlice("gidmap"), c.String("subuidname"), c.String("subgidname"))
	if err != nil {
		return nil, err
	}

	imageID := ""

	inputCommand = c.InputArgs[1:]
	if data != nil {
		imageID = data.ID
	}

	rootfs := ""
	if c.Bool("rootfs") {
		rootfs = c.InputArgs[0]
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

	tty := c.Bool("tty")

	if c.Changed("cpu-period") && c.Changed("cpus") {
		return nil, errors.Errorf("--cpu-period and --cpus cannot be set together")
	}
	if c.Changed("cpu-quota") && c.Changed("cpus") {
		return nil, errors.Errorf("--cpu-quota and --cpus cannot be set together")
	}

	if c.Bool("no-hosts") && c.Changed("add-host") {
		return nil, errors.Errorf("--no-hosts and --add-host cannot be set together")
	}

	// EXPOSED PORTS
	var portBindings map[nat.Port][]nat.PortBinding
	if data != nil {
		portBindings, err = cc.ExposedPorts(c.StringSlice("expose"), c.StringSlice("publish"), c.Bool("publish-all"), data.Config.ExposedPorts)
		if err != nil {
			return nil, err
		}
	}

	// Kernel Namespaces
	// TODO Fix handling of namespace from pod
	// Instead of integrating here, should be done in libpod
	// However, that also involves setting up security opts
	// when the pod's namespace is integrated
	namespaceNet := c.String("network")
	if c.Changed("net") {
		namespaceNet = c.String("net")
	}
	namespaces = map[string]string{
		"cgroup": c.String("cgroupns"),
		"pid":    c.String("pid"),
		"net":    namespaceNet,
		"ipc":    c.String("ipc"),
		"user":   c.String("userns"),
		"uts":    c.String("uts"),
	}

	originalPodName := c.String("pod")
	podName := strings.Replace(originalPodName, "new:", "", 1)
	// after we strip out :new, make sure there is something left for a pod name
	if len(podName) < 1 && c.IsSet("pod") {
		return nil, errors.Errorf("new pod name must be at least one character")
	}
	if c.IsSet("pod") {
		if strings.HasPrefix(originalPodName, "new:") {
			// pod does not exist; lets make it
			var podOptions []libpod.PodCreateOption
			podOptions = append(podOptions, libpod.WithPodName(podName), libpod.WithInfraContainer(), libpod.WithPodCgroups())
			if len(portBindings) > 0 {
				ociPortBindings, err := cc.NatToOCIPortBindings(portBindings)
				if err != nil {
					return nil, err
				}
				podOptions = append(podOptions, libpod.WithInfraContainerPorts(ociPortBindings))
			}

			podNsOptions, err := GetNamespaceOptions(strings.Split(DefaultKernelNamespaces, ","))
			if err != nil {
				return nil, err
			}
			podOptions = append(podOptions, podNsOptions...)
			// make pod
			pod, err := runtime.NewPod(ctx, podOptions...)
			if err != nil {
				return nil, err
			}
			logrus.Debugf("pod %s created by new container request", pod.ID())

			// The container now cannot have port bindings; so we reset the map
			portBindings = make(map[nat.Port][]nat.PortBinding)
		}
		namespaces, err = configurePod(c, runtime, namespaces, podName)
		if err != nil {
			return nil, err
		}
	}

	pidMode := ns.PidMode(namespaces["pid"])
	if !cc.Valid(string(pidMode), pidMode) {
		return nil, errors.Errorf("--pid %q is not valid", c.String("pid"))
	}

	usernsMode := ns.UsernsMode(namespaces["user"])
	if !cc.Valid(string(usernsMode), usernsMode) {
		return nil, errors.Errorf("--userns %q is not valid", namespaces["user"])
	}

	utsMode := ns.UTSMode(namespaces["uts"])
	if !cc.Valid(string(utsMode), utsMode) {
		return nil, errors.Errorf("--uts %q is not valid", namespaces["uts"])
	}

	cgroupMode := ns.CgroupMode(namespaces["cgroup"])
	if !cgroupMode.Valid() {
		return nil, errors.Errorf("--cgroup %q is not valid", namespaces["cgroup"])
	}

	ipcMode := ns.IpcMode(namespaces["ipc"])
	if !cc.Valid(string(ipcMode), ipcMode) {
		return nil, errors.Errorf("--ipc %q is not valid", ipcMode)
	}

	// Make sure if network is set to container namespace, port binding is not also being asked for
	netMode := ns.NetworkMode(namespaces["net"])
	if netMode.IsContainer() {
		if len(portBindings) > 0 {
			return nil, errors.Errorf("cannot set port bindings on an existing container network namespace")
		}
	}

	// USER
	user := c.String("user")
	if user == "" {
		if usernsMode.IsKeepID() {
			user = fmt.Sprintf("%d:%d", rootless.GetRootlessUID(), rootless.GetRootlessGID())
		} else if data == nil {
			user = "0"
		} else {
			user = data.Config.User
		}
	}

	// STOP SIGNAL
	stopSignal := syscall.SIGTERM
	signalString := ""
	if data != nil {
		signalString = data.Config.StopSignal
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
	env := EnvVariablesFromData(data)
	if c.Bool("env-host") {
		for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			if _, ok := env[pair[0]]; !ok {
				if len(pair) > 1 {
					env[pair[0]] = pair[1]
				}
			}
		}
	}
	if err := parse.ReadKVStrings(env, c.StringSlice("env-file"), c.StringArray("env")); err != nil {
		return nil, errors.Wrapf(err, "unable to process environment variables")
	}

	// LABEL VARIABLES
	labels, err := GetAllLabels(c.StringSlice("label-file"), c.StringArray("label"))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to process labels")
	}
	if data != nil {
		for key, val := range data.Config.Labels {
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
	if c.IsSet("workdir") {
		workDir = c.String("workdir")
	} else if data != nil && data.Config.WorkingDir != "" {
		workDir = data.Config.WorkingDir
	}

	userCommand := []string{}
	entrypoint := configureEntrypoint(c, data)
	// Build the command
	// If we have an entry point, it goes first
	if len(entrypoint) > 0 {
		command = entrypoint
	}
	if len(inputCommand) > 0 {
		// User command overrides data CMD
		command = append(command, inputCommand...)
		userCommand = append(userCommand, inputCommand...)
	} else if data != nil && len(data.Config.Cmd) > 0 && !c.IsSet("entrypoint") {
		// If not user command, add CMD
		command = append(command, data.Config.Cmd...)
		userCommand = append(userCommand, data.Config.Cmd...)
	}

	if data != nil && len(command) == 0 {
		return nil, errors.Errorf("No command specified on command line or as CMD or ENTRYPOINT in this image")
	}

	// SHM Size
	shmSize, err := units.FromHumanSize(c.String("shm-size"))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to translate --shm-size")
	}

	// Verify the additional hosts are in correct format
	for _, host := range c.StringSlice("add-host") {
		if _, err := parse.ValidateExtraHost(host); err != nil {
			return nil, err
		}
	}

	// Check for . and dns-search domains
	if util.StringInSlice(".", c.StringSlice("dns-search")) && len(c.StringSlice("dns-search")) > 1 {
		return nil, errors.Errorf("cannot pass additional search domains when also specifying '.'")
	}

	// Check for explicit dns-search domain of ''
	if c.Changed("dns-search") && len(c.StringSlice("dns-search")) == 0 {
		return nil, errors.Errorf("'' is not a valid domain")
	}

	// Validate domains are good
	for _, dom := range c.StringSlice("dns-search") {
		if dom == "." {
			continue
		}
		if _, err := parse.ValidateDomain(dom); err != nil {
			return nil, err
		}
	}

	var ImageVolumes map[string]struct{}
	if data != nil && c.String("image-volume") != "ignore" {
		ImageVolumes = data.Config.Volumes
	}

	var imageVolType = map[string]string{
		"bind":   "",
		"tmpfs":  "",
		"ignore": "",
	}
	if _, ok := imageVolType[c.String("image-volume")]; !ok {
		return nil, errors.Errorf("invalid image-volume type %q. Pick one of bind, tmpfs, or ignore", c.String("image-volume"))
	}

	systemd := c.String("systemd") == "always"
	if !systemd && command != nil {
		x, err := strconv.ParseBool(c.String("systemd"))
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse bool %s", c.String("systemd"))
		}
		if x && (command[0] == "/usr/sbin/init" || command[0] == "/sbin/init" || (filepath.Base(command[0]) == "systemd")) {
			systemd = true
		}
	}
	if systemd {
		if signalString == "" {
			stopSignal, err = signal.ParseSignal("RTMIN+3")
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing systemd signal")
			}
		}
	}
	// This is done because cobra cannot have two aliased flags. So we have to check
	// both
	network := c.String("network")
	if c.Changed("net") {
		network = c.String("net")
	}

	memorySwappiness := c.Int64("memory-swappiness")

	logDriver := libpod.KubernetesLogging
	if c.Changed("log-driver") {
		logDriver = c.String("log-driver")
	}

	pidsLimit := c.Int64("pids-limit")
	if c.String("cgroups") == "disabled" && !c.Changed("pids-limit") {
		pidsLimit = 0
	}

	pid := &cc.PidConfig{
		PidMode: pidMode,
	}
	ipc := &cc.IpcConfig{
		IpcMode: ipcMode,
	}

	cgroup := &cc.CgroupConfig{
		Cgroups:      c.String("cgroups"),
		Cgroupns:     c.String("cgroupns"),
		CgroupParent: c.String("cgroup-parent"),
		CgroupMode:   cgroupMode,
	}

	userns := &cc.UserConfig{
		GroupAdd:   c.StringSlice("group-add"),
		IDMappings: idmappings,
		UsernsMode: usernsMode,
		User:       user,
	}

	uts := &cc.UtsConfig{
		UtsMode:  utsMode,
		NoHosts:  c.Bool("no-hosts"),
		HostAdd:  c.StringSlice("add-host"),
		Hostname: c.String("hostname"),
	}

	net := &cc.NetworkConfig{
		DNSOpt:       c.StringSlice("dns-opt"),
		DNSSearch:    c.StringSlice("dns-search"),
		DNSServers:   c.StringSlice("dns"),
		HTTPProxy:    c.Bool("http-proxy"),
		MacAddress:   c.String("mac-address"),
		Network:      network,
		NetMode:      netMode,
		IPAddress:    c.String("ip"),
		Publish:      c.StringSlice("publish"),
		PublishAll:   c.Bool("publish-all"),
		PortBindings: portBindings,
	}

	sysctl, err := validateSysctl(c.StringSlice("sysctl"))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid value for sysctl")
	}

	secConfig := &cc.SecurityConfig{
		CapAdd:         c.StringSlice("cap-add"),
		CapDrop:        c.StringSlice("cap-drop"),
		Privileged:     c.Bool("privileged"),
		ReadOnlyRootfs: c.Bool("read-only"),
		ReadOnlyTmpfs:  c.Bool("read-only-tmpfs"),
		Sysctl:         sysctl,
	}

	if err := secConfig.SetLabelOpts(runtime, pid, ipc); err != nil {
		return nil, err
	}
	if err := secConfig.SetSecurityOpts(runtime, c.StringArray("security-opt")); err != nil {
		return nil, err
	}

	config := &cc.CreateConfig{
		Annotations:       annotations,
		BuiltinImgVolumes: ImageVolumes,
		ConmonPidFile:     c.String("conmon-pidfile"),
		ImageVolumeType:   c.String("image-volume"),
		CidFile:           c.String("cidfile"),
		Command:           command,
		UserCommand:       userCommand,
		Detach:            c.Bool("detach"),
		Devices:           c.StringSlice("device"),
		Entrypoint:        entrypoint,
		Env:               env,
		// ExposedPorts:   ports,
		Init:        c.Bool("init"),
		InitPath:    c.String("init-path"),
		Image:       imageName,
		ImageID:     imageID,
		Interactive: c.Bool("interactive"),
		// IP6Address:     c.String("ipv6"), // Not implemented yet - needs CNI support for static v6
		Labels: labels,
		// LinkLocalIP:    c.StringSlice("link-local-ip"), // Not implemented yet
		LogDriver:    logDriver,
		LogDriverOpt: c.StringSlice("log-opt"),
		Name:         c.String("name"),
		// NetworkAlias:   c.StringSlice("network-alias"), // Not implemented - does this make sense in Podman?
		Pod:   podName,
		Quiet: c.Bool("quiet"),
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
			MemorySwappiness:  int(memorySwappiness),
			KernelMemory:      memoryKernel,
			OomScoreAdj:       c.Int("oom-score-adj"),
			PidsLimit:         pidsLimit,
			Ulimit:            c.StringSlice("ulimit"),
		},
		RestartPolicy: c.String("restart"),
		Rm:            c.Bool("rm"),
		Security:      *secConfig,
		StopSignal:    stopSignal,
		StopTimeout:   c.Uint("stop-timeout"),
		Systemd:       systemd,
		Tmpfs:         c.StringArray("tmpfs"),
		Tty:           tty,
		MountsFlag:    c.StringArray("mount"),
		Volumes:       c.StringArray("volume"),
		WorkDir:       workDir,
		Rootfs:        rootfs,
		VolumesFrom:   c.StringSlice("volumes-from"),
		Syslog:        c.Bool("syslog"),

		Pid:     *pid,
		Ipc:     *ipc,
		Cgroup:  *cgroup,
		User:    *userns,
		Uts:     *uts,
		Network: *net,
	}

	warnings, err := verifyContainerResources(config, false)
	if err != nil {
		return nil, err
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, warning)
	}
	return config, nil
}

func CreateContainerFromCreateConfig(r *libpod.Runtime, createConfig *cc.CreateConfig, ctx context.Context, pod *libpod.Pod) (*libpod.Container, error) {
	runtimeSpec, options, err := createConfig.MakeContainerConfig(r, pod)
	if err != nil {
		return nil, err
	}

	ctr, err := r.NewContainer(ctx, runtimeSpec, options...)
	if err != nil {
		return nil, err
	}
	return ctr, nil
}

var defaultEnvVariables = map[string]string{
	"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	"TERM": "xterm",
}

// EnvVariablesFromData gets sets the default environment variables
// for containers, and reads the variables from the image data, if present.
func EnvVariablesFromData(data *inspect.ImageData) map[string]string {
	env := defaultEnvVariables
	if data != nil {
		for _, e := range data.Config.Env {
			split := strings.SplitN(e, "=", 2)
			if len(split) > 1 {
				env[split[0]] = split[1]
			} else {
				env[split[0]] = ""
			}
		}
	}
	return env
}

func makeHealthCheckFromCli(c *GenericCLIResults) (*manifest.Schema2HealthConfig, error) {
	inCommand := c.String("healthcheck-command")
	inInterval := c.String("healthcheck-interval")
	inRetries := c.Uint("healthcheck-retries")
	inTimeout := c.String("healthcheck-timeout")
	inStartPeriod := c.String("healthcheck-start-period")

	// Every healthcheck requires a command
	if len(inCommand) == 0 {
		return nil, errors.New("Must define a healthcheck command for all healthchecks")
	}

	// first try to parse option value as JSON array of strings...
	cmd := []string{}
	err := json.Unmarshal([]byte(inCommand), &cmd)
	if err != nil {
		// ...otherwise pass it to "/bin/sh -c" inside the container
		cmd = []string{"CMD-SHELL", inCommand}
	}
	hc := manifest.Schema2HealthConfig{
		Test: cmd,
	}

	if inInterval == "disable" {
		inInterval = "0"
	}
	intervalDuration, err := time.ParseDuration(inInterval)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-interval %s ", inInterval)
	}

	hc.Interval = intervalDuration

	if inRetries < 1 {
		return nil, errors.New("healthcheck-retries must be greater than 0.")
	}
	hc.Retries = int(inRetries)
	timeoutDuration, err := time.ParseDuration(inTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-timeout %s", inTimeout)
	}
	if timeoutDuration < time.Duration(1) {
		return nil, errors.New("healthcheck-timeout must be at least 1 second")
	}
	hc.Timeout = timeoutDuration

	startPeriodDuration, err := time.ParseDuration(inStartPeriod)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid healthcheck-start-period %s", inStartPeriod)
	}
	if startPeriodDuration < time.Duration(0) {
		return nil, errors.New("healthcheck-start-period must be 0 seconds or greater")
	}
	hc.StartPeriod = startPeriodDuration

	return &hc, nil
}
