package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	ann "github.com/containers/libpod/pkg/annotations"
	"github.com/containers/libpod/pkg/inspect"
	ns "github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/libpod/pkg/rootless"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	createCommand     cliconfig.CreateValues
	createDescription = "Creates a new container from the given image or" +
		" storage and prepares it for running the specified command. The" +
		" container ID is then printed to stdout. You can then start it at" +
		" any time with the podman start <container_id> command. The container" +
		" will be created with the initial state 'created'."
	_createCommand = &cobra.Command{
		Use:   "create [flags] IMAGE [COMMAND [ARG...]]",
		Short: "Create but do not start a container",
		Long:  createDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			createCommand.InputArgs = args
			createCommand.GlobalFlags = MainGlobalOpts
			return createCmd(&createCommand)
		},
		Example: `podman create alpine ls
  podman create --annotation HELLO=WORLD alpine ls
  podman create -t -i --name myctr alpine ls`,
	}

	defaultEnvVariables = map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM": "xterm",
	}
)

func init() {
	createCommand.PodmanCommand.Command = _createCommand
	createCommand.SetUsageTemplate(UsageTemplate())

	getCreateFlags(&createCommand.PodmanCommand)
	flags := createCommand.Flags()
	flags.SetInterspersed(true)

}

func createCmd(c *cliconfig.CreateValues) error {
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "createCmd")
		defer span.Finish()
	}

	if err := createInit(&c.PodmanCommand); err != nil {
		return err
	}

	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	ctr, _, err := createContainer(&c.PodmanCommand, runtime)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", ctr.ID())
	return nil
}

func createInit(c *cliconfig.PodmanCommand) error {
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "createInit")
		defer span.Finish()
	}

	// Docker-compatibility: the "-h" flag for run/create is reserved for
	// the hostname (see https://github.com/containers/libpod/issues/1367).

	if len(c.InputArgs) < 1 {
		return errors.Errorf("image name or ID is required")
	}

	return nil
}

func createContainer(c *cliconfig.PodmanCommand, runtime *libpod.Runtime) (*libpod.Container, *cc.CreateConfig, error) {
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "createContainer")
		defer span.Finish()
	}

	rtc := runtime.GetConfig()
	ctx := getContext()
	rootfs := ""
	if c.Bool("rootfs") {
		rootfs = c.InputArgs[0]
	}

	var err error
	var cidFile *os.File
	if c.IsSet("cidfile") && os.Geteuid() == 0 {
		cidFile, err = libpod.OpenExclusiveFile(c.String("cidfile"))
		if err != nil && os.IsExist(err) {
			return nil, nil, errors.Errorf("container id file exists. Ensure another container is not using it or delete %s", c.String("cidfile"))
		}
		if err != nil {
			return nil, nil, errors.Errorf("error opening cidfile %s", c.String("cidfile"))
		}
		defer cidFile.Close()
		defer cidFile.Sync()
	}

	imageName := ""
	var data *inspect.ImageData = nil

	if rootfs == "" && !rootless.SkipStorageSetup() {
		var writer io.Writer
		if !c.Bool("quiet") {
			writer = os.Stderr
		}

		newImage, err := runtime.ImageRuntime().New(ctx, c.InputArgs[0], rtc.SignaturePolicyPath, "", writer, nil, image.SigningOptions{}, false, nil)
		if err != nil {
			return nil, nil, err
		}
		data, err = newImage.Inspect(ctx)
		names := newImage.Names()
		if len(names) > 0 {
			imageName = names[0]
		} else {
			imageName = newImage.ID()
		}
	}
	createConfig, err := parseCreateOpts(ctx, c, runtime, imageName, data)
	if err != nil {
		return nil, nil, err
	}

	ctr, err := createContainerFromCreateConfig(runtime, createConfig, ctx, nil)
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

func parseSecurityOpt(config *cc.CreateConfig, securityOpts []string) error {
	var (
		labelOpts []string
	)

	if config.PidMode.IsHost() {
		labelOpts = append(labelOpts, label.DisableSecOpt()...)
	} else if config.PidMode.IsContainer() {
		ctr, err := config.Runtime.LookupContainer(config.PidMode.Container())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", config.PidMode.Container())
		}
		secopts, err := label.DupSecOpt(ctr.ProcessLabel())
		if err != nil {
			return errors.Wrapf(err, "failed to duplicate label %q ", ctr.ProcessLabel())
		}
		labelOpts = append(labelOpts, secopts...)
	}

	if config.IpcMode.IsHost() {
		labelOpts = append(labelOpts, label.DisableSecOpt()...)
	} else if config.IpcMode.IsContainer() {
		ctr, err := config.Runtime.LookupContainer(config.IpcMode.Container())
		if err != nil {
			return errors.Wrapf(err, "container %q not found", config.IpcMode.Container())
		}
		secopts, err := label.DupSecOpt(ctr.ProcessLabel())
		if err != nil {
			return errors.Wrapf(err, "failed to duplicate label %q ", ctr.ProcessLabel())
		}
		labelOpts = append(labelOpts, secopts...)
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
	config.LabelOpts = labelOpts
	return nil
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

func configureEntrypoint(c *cliconfig.PodmanCommand, data *inspect.ImageData) []string {
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

func configurePod(c *cliconfig.PodmanCommand, runtime *libpod.Runtime, namespaces map[string]string, podName string) (map[string]string, error) {
	pod, err := runtime.LookupPod(podName)
	if err != nil {
		return namespaces, err
	}
	podInfraID, err := pod.InfraContainerID()
	if err != nil {
		return namespaces, err
	}
	if (namespaces["pid"] == cc.Pod) || (!c.IsSet("pid") && pod.SharesPID()) {
		namespaces["pid"] = fmt.Sprintf("container:%s", podInfraID)
	}
	if (namespaces["net"] == cc.Pod) || (!c.IsSet("net") && !c.IsSet("network") && pod.SharesNet()) {
		namespaces["net"] = fmt.Sprintf("container:%s", podInfraID)
	}
	if (namespaces["user"] == cc.Pod) || (!c.IsSet("user") && pod.SharesUser()) {
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
func parseCreateOpts(ctx context.Context, c *cliconfig.PodmanCommand, runtime *libpod.Runtime, imageName string, data *inspect.ImageData) (*cc.CreateConfig, error) {
	var (
		inputCommand, command                                    []string
		memoryLimit, memoryReservation, memorySwap, memoryKernel int64
		blkioWeight                                              uint16
		namespaces                                               map[string]string
	)
	if c.IsSet("restart") {
		return nil, errors.Errorf("--restart option is not supported.\nUse systemd unit files for restarting containers")
	}

	idmappings, err := util.ParseIDMapping(c.StringSlice("uidmap"), c.StringSlice("gidmap"), c.String("subuidname"), c.String("subgidname"))
	if err != nil {
		return nil, err
	}

	if c.String("mac-address") != "" {
		return nil, errors.Errorf("--mac-address option not currently supported")
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
	var mountList []spec.Mount
	if mountList, err = parseMounts(c.StringArray("mount")); err != nil {
		return nil, err
	}

	if err = parseVolumes(c.StringArray("volume")); err != nil {
		return nil, err
	}

	if err = parseVolumesFrom(c.StringSlice("volumes-from")); err != nil {
		return nil, err
	}

	tty := c.Bool("tty")

	if c.Flag("cpu-period").Changed && c.Flag("cpus").Changed {
		return nil, errors.Errorf("--cpu-period and --cpus cannot be set together")
	}
	if c.Flag("cpu-quota").Changed && c.Flag("cpus").Changed {
		return nil, errors.Errorf("--cpu-quota and --cpus cannot be set together")
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
	if c.Flag("net").Changed {
		namespaceNet = c.String("net")
	}
	namespaces = map[string]string{
		"pid":  c.String("pid"),
		"net":  namespaceNet,
		"ipc":  c.String("ipc"),
		"user": c.String("userns"),
		"uts":  c.String("uts"),
	}

	originalPodName := c.String("pod")
	podName := strings.Replace(originalPodName, "new:", "", 1)
	// after we strip out :new, make sure there is something left for a pod name
	if len(podName) < 1 && c.IsSet("pod") {
		return nil, errors.Errorf("new pod name must be at least one character")
	}
	if c.IsSet("pod") {
		if strings.HasPrefix(originalPodName, "new:") {
			if rootless.IsRootless() {
				// To create a new pod, we must immediately create the userns.
				became, ret, err := rootless.BecomeRootInUserNS()
				if err != nil {
					return nil, err
				}
				if became {
					os.Exit(ret)
				}
			}
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

			podNsOptions, err := shared.GetNamespaceOptions(strings.Split(DefaultKernelNamespaces, ","))
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
		if data == nil {
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
	if err := readKVStrings(env, c.StringSlice("env-file"), c.StringSlice("env")); err != nil {
		return nil, errors.Wrapf(err, "unable to process environment variables")
	}

	// LABEL VARIABLES
	labels, err := getAllLabels(c.StringSlice("label-file"), c.StringSlice("label"))
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
	if c.IsSet("workdir") || c.IsSet("w") {
		workDir = c.String("workdir")
	} else if data != nil && data.Config.WorkingDir != "" {
		workDir = data.Config.WorkingDir
	}

	entrypoint := configureEntrypoint(c, data)
	// Build the command
	// If we have an entry point, it goes first
	if len(entrypoint) > 0 {
		command = entrypoint
	}
	if len(inputCommand) > 0 {
		// User command overrides data CMD
		command = append(command, inputCommand...)
	} else if data != nil && len(data.Config.Cmd) > 0 && !c.IsSet("entrypoint") {
		// If not user command, add CMD
		command = append(command, data.Config.Cmd...)
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

	var systemd bool
	if command != nil && c.Bool("systemd") && ((filepath.Base(command[0]) == "init") || (filepath.Base(command[0]) == "systemd")) {
		systemd = true
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
	if c.Flag("net").Changed {
		network = c.String("net")
	}

	var memorySwappiness int64
	if c.Flags().Lookup("memory-swappiness") != nil {
		memorySwappiness, _ = c.Flags().GetInt64("memory-swappiness")
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
		GroupAdd:    c.StringSlice("group-add"),
		Hostname:    c.String("hostname"),
		HostAdd:     c.StringSlice("add-host"),
		IDMappings:  idmappings,
		Image:       imageName,
		ImageID:     imageID,
		Interactive: c.Bool("interactive"),
		//IP6Address:     c.String("ipv6"), // Not implemented yet - needs CNI support for static v6
		IPAddress: c.String("ip"),
		Labels:    labels,
		//LinkLocalIP:    c.StringSlice("link-local-ip"), // Not implemented yet
		LogDriver:    c.String("log-driver"),
		LogDriverOpt: c.StringSlice("log-opt"),
		MacAddress:   c.String("mac-address"),
		Name:         c.String("name"),
		Network:      network,
		//NetworkAlias:   c.StringSlice("network-alias"), // Not implemented - does this make sense in Podman?
		IpcMode:        ipcMode,
		NetMode:        netMode,
		UtsMode:        utsMode,
		PidMode:        pidMode,
		Pod:            podName,
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
			MemorySwappiness:  int(memorySwappiness),
			KernelMemory:      memoryKernel,
			OomScoreAdj:       c.Int("oom-score-adj"),
			PidsLimit:         c.Int64("pids-limit"),
			Ulimit:            c.StringSlice("ulimit"),
		},
		Rm:          c.Bool("rm"),
		StopSignal:  stopSignal,
		StopTimeout: c.Uint("stop-timeout"),
		Sysctl:      sysctl,
		Systemd:     systemd,
		Tmpfs:       c.StringSlice("tmpfs"),
		Tty:         tty,
		User:        user,
		UsernsMode:  usernsMode,
		Mounts:      mountList,
		Volumes:     c.StringArray("volume"),
		WorkDir:     workDir,
		Rootfs:      rootfs,
		VolumesFrom: c.StringSlice("volumes-from"),
		Syslog:      c.GlobalFlags.Syslog,
	}
	if c.Bool("init") {
		initPath := c.String("init-path")
		if initPath == "" {
			initPath = runtime.GetConfig().InitPath
		}
		if err := config.AddContainerInitBinary(initPath); err != nil {
			return nil, err
		}
	}

	if config.Privileged {
		config.LabelOpts = label.DisableSecOpt()
	} else {
		if err := parseSecurityOpt(config, c.StringArray("security-opt")); err != nil {
			return nil, err
		}
	}
	config.SecurityOpts = c.StringArray("security-opt")
	warnings, err := verifyContainerResources(config, false)
	if err != nil {
		return nil, err
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, warning)
	}
	return config, nil
}

type namespace interface {
	IsContainer() bool
	Container() string
}

func joinOrCreateRootlessUserNamespace(createConfig *cc.CreateConfig, runtime *libpod.Runtime) (bool, int, error) {
	if os.Geteuid() == 0 {
		return false, 0, nil
	}

	if createConfig.Pod != "" {
		pod, err := runtime.LookupPod(createConfig.Pod)
		if err != nil {
			return false, -1, err
		}
		inspect, err := pod.Inspect()
		for _, ctr := range inspect.Containers {
			prevCtr, err := runtime.LookupContainer(ctr.ID)
			if err != nil {
				return false, -1, err
			}
			s, err := prevCtr.State()
			if err != nil {
				return false, -1, err
			}
			if s != libpod.ContainerStateRunning && s != libpod.ContainerStatePaused {
				continue
			}
			data, err := ioutil.ReadFile(prevCtr.Config().ConmonPidFile)
			if err != nil {
				return false, -1, errors.Wrapf(err, "cannot read conmon PID file %q", prevCtr.Config().ConmonPidFile)
			}
			conmonPid, err := strconv.Atoi(string(data))
			if err != nil {
				return false, -1, errors.Wrapf(err, "cannot parse PID %q", data)
			}
			return rootless.JoinDirectUserAndMountNS(uint(conmonPid))
		}
	}

	namespacesStr := []string{string(createConfig.IpcMode), string(createConfig.NetMode), string(createConfig.UsernsMode), string(createConfig.PidMode), string(createConfig.UtsMode)}
	for _, i := range namespacesStr {
		if cc.IsNS(i) {
			return rootless.JoinNSPath(cc.NS(i))
		}
	}

	namespaces := []namespace{createConfig.IpcMode, createConfig.NetMode, createConfig.UsernsMode, createConfig.PidMode, createConfig.UtsMode}
	for _, i := range namespaces {
		if i.IsContainer() {
			ctr, err := runtime.LookupContainer(i.Container())
			if err != nil {
				return false, -1, err
			}
			pid, err := ctr.PID()
			if err != nil {
				return false, -1, err
			}
			if pid == 0 {
				if createConfig.Pod != "" {
					continue
				}
				return false, -1, errors.Errorf("dependency container %s is not running", ctr.ID())
			}
			return rootless.JoinNS(uint(pid))
		}
	}
	return rootless.BecomeRootInUserNS()
}

func createContainerFromCreateConfig(r *libpod.Runtime, createConfig *cc.CreateConfig, ctx context.Context, pod *libpod.Pod) (*libpod.Container, error) {
	runtimeSpec, err := cc.CreateConfigToOCISpec(createConfig)
	if err != nil {
		return nil, err
	}

	options, err := createConfig.GetContainerCreateOptions(r, pod)
	if err != nil {
		return nil, err
	}
	became, ret, err := joinOrCreateRootlessUserNamespace(createConfig, r)
	if err != nil {
		return nil, err
	}
	if became {
		os.Exit(ret)
	}

	ctr, err := r.NewContainer(ctx, runtimeSpec, options...)
	if err != nil {
		return nil, err
	}

	createConfigJSON, err := json.Marshal(createConfig)
	if err != nil {
		return nil, err
	}
	if err := ctr.AddArtifact("create-config", createConfigJSON); err != nil {
		return nil, err
	}
	return ctr, nil
}
