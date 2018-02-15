package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/pkg/inspect"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type mountType string

// Type constants
const (
	// TypeBind is the type for mounting host dir
	TypeBind mountType = "bind"
	// TypeVolume is the type for remote storage volumes
	// TypeVolume mountType = "volume"  // re-enable upon use
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs mountType = "tmpfs"
)

var (
	defaultEnvVariables = map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM": "xterm",
	}
)

type createResourceConfig struct {
	BlkioWeight       uint16   // blkio-weight
	BlkioWeightDevice []string // blkio-weight-device
	CPUPeriod         uint64   // cpu-period
	CPUQuota          int64    // cpu-quota
	CPURtPeriod       uint64   // cpu-rt-period
	CPURtRuntime      int64    // cpu-rt-runtime
	CPUShares         uint64   // cpu-shares
	CPUs              float64  // cpus
	CPUsetCPUs        string
	CPUsetMems        string   // cpuset-mems
	DeviceReadBps     []string // device-read-bps
	DeviceReadIOps    []string // device-read-iops
	DeviceWriteBps    []string // device-write-bps
	DeviceWriteIOps   []string // device-write-iops
	DisableOomKiller  bool     // oom-kill-disable
	KernelMemory      int64    // kernel-memory
	Memory            int64    //memory
	MemoryReservation int64    // memory-reservation
	MemorySwap        int64    //memory-swap
	MemorySwappiness  int      // memory-swappiness
	OomScoreAdj       int      //oom-score-adj
	PidsLimit         int64    // pids-limit
	ShmSize           int64
	Ulimit            []string //ulimit
}

type createConfig struct {
	Runtime            *libpod.Runtime
	Args               []string
	CapAdd             []string // cap-add
	CapDrop            []string // cap-drop
	CidFile            string
	CgroupParent       string // cgroup-parent
	Command            []string
	Detach             bool              // detach
	Devices            []string          // device
	DNSOpt             []string          //dns-opt
	DNSSearch          []string          //dns-search
	DNSServers         []string          //dns
	Entrypoint         []string          //entrypoint
	Env                map[string]string //env
	ExposedPorts       map[nat.Port]struct{}
	GroupAdd           []uint32 // group-add
	HostAdd            []string //add-host
	Hostname           string   //hostname
	Image              string
	ImageID            string
	BuiltinImgVolumes  map[string]struct{}   // volumes defined in the image config
	ImageVolumeType    string                // how to handle the image volume, either bind, tmpfs, or ignore
	Interactive        bool                  //interactive
	IpcMode            container.IpcMode     //ipc
	IP6Address         string                //ipv6
	IPAddress          string                //ip
	Labels             map[string]string     //label
	LinkLocalIP        []string              // link-local-ip
	LogDriver          string                // log-driver
	LogDriverOpt       []string              // log-opt
	MacAddress         string                //mac-address
	Name               string                //name
	NetMode            container.NetworkMode //net
	Network            string                //network
	NetworkAlias       []string              //network-alias
	PidMode            container.PidMode     //pid
	NsUser             string
	Pod                string //pod
	PortBindings       nat.PortMap
	Privileged         bool     //privileged
	Publish            []string //publish
	PublishAll         bool     //publish-all
	Quiet              bool     //quiet
	ReadOnlyRootfs     bool     //read-only
	Resources          createResourceConfig
	Rm                 bool //rm
	ShmDir             string
	SigProxy           bool              //sig-proxy
	StopSignal         syscall.Signal    // stop-signal
	StopTimeout        uint              // stop-timeout
	Sysctl             map[string]string //sysctl
	Tmpfs              []string          // tmpfs
	Tty                bool              //tty
	User               string            //user
	UtsMode            container.UTSMode //uts
	Volumes            []string          //volume
	WorkDir            string            //workdir
	MountLabel         string            //SecurityOpts
	ProcessLabel       string            //SecurityOpts
	NoNewPrivs         bool              //SecurityOpts
	ApparmorProfile    string            //SecurityOpts
	SeccompProfilePath string            //SecurityOpts
	SecurityOpts       []string
}

var createDescription = "Creates a new container from the given image or" +
	" storage and prepares it for running the specified command. The" +
	" container ID is then printed to stdout. You can then start it at" +
	" any time with the podman start <container_id> command. The container" +
	" will be created with the initial state 'created'."

var createCommand = cli.Command{
	Name:                   "create",
	Usage:                  "create but do not start a container",
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
		if err := libpod.WriteFile("", c.String("cidfile")); err != nil {
			return errors.Wrapf(err, "unable to write cidfile %s", c.String("cidfile"))
		}
	}

	if len(c.Args()) < 1 {
		return errors.Errorf("image name or ID is required")
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	imageName, _, data, err := imageData(c, runtime, c.Args()[0])
	if err != nil {
		return err
	}
	createConfig, err := parseCreateOpts(c, runtime, imageName, data)
	if err != nil {
		return err
	}
	useImageVolumes := createConfig.ImageVolumeType == "bind"

	runtimeSpec, err := createConfigToOCISpec(createConfig)
	if err != nil {
		return err
	}
	options, err := createConfig.GetContainerCreateOptions()
	if err != nil {
		return errors.Wrapf(err, "unable to parse new container options")
	}
	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(createConfig.ImageID, createConfig.Image, useImageVolumes))
	options = append(options, libpod.WithSELinuxLabels(createConfig.ProcessLabel, createConfig.MountLabel))
	options = append(options, libpod.WithLabels(createConfig.Labels))
	options = append(options, libpod.WithUser(createConfig.User))
	options = append(options, libpod.WithShmDir(createConfig.ShmDir))
	options = append(options, libpod.WithShmSize(createConfig.Resources.ShmSize))
	ctr, err := runtime.NewContainer(runtimeSpec, options...)
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

func parseSecurityOpt(config *createConfig, securityOpts []string) error {
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
	return libpod.StringInSlice(port.Port(), hostPorts)
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

func exposedPorts(c *cli.Context, imageExposedPorts map[string]struct{}) (map[nat.Port][]nat.PortBinding, error) {
	containerPorts := make(map[string]string)

	// add expose ports from the image itself
	for expose := range imageExposedPorts {
		_, port := nat.SplitProtoPort(expose)
		containerPorts[port] = ""
	}

	// add the expose ports from the user (--expose)
	// can be single or a range
	for _, expose := range c.StringSlice("expose") {
		//support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		_, port := nat.SplitProtoPort(expose)
		//parse the start and end port and create a sequence of ports to expose
		//if expose a port, the start and end port are the same
		start, end, err := nat.ParsePortRange(port)
		if err != nil {
			return nil, fmt.Errorf("invalid range format for --expose: %s, error: %s", expose, err)
		}
		for i := start; i <= end; i++ {
			containerPorts[strconv.Itoa(int(i))] = ""
		}
	}

	// parse user input'd port bindings
	pbPorts, portBindings, err := nat.ParsePortSpecs(c.StringSlice("publish"))
	if err != nil {
		return nil, err
	}

	// delete exposed container ports if being used by -p
	for i := range pbPorts {
		delete(containerPorts, i.Port())
	}

	// iterate container ports and make port bindings from them
	if c.Bool("publish-all") {
		for e := range containerPorts {
			//support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
			//proto, port := nat.SplitProtoPort(e)
			p, err := nat.NewPort("tcp", e)
			if err != nil {
				return nil, err
			}
			l, err := net.Listen("tcp", ":0")
			if err != nil {
				return nil, errors.Wrapf(err, "unable to get free port")
			}
			defer l.Close()
			_, randomPort, err := net.SplitHostPort(l.Addr().String())
			if err != nil {
				return nil, errors.Wrapf(err, "unable to determine free port")
			}
			rp, err := strconv.Atoi(randomPort)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to convert random port to int")
			}
			logrus.Debug(fmt.Sprintf("Using random host port %s with container port %d", randomPort, p.Int()))
			portBindings[p] = CreatePortBinding(rp, "")
		}
	}
	return portBindings, nil
}

// imageData pulls down the image if not stored locally and extracts the
// default container runtime data out of it. imageData returns the data
// to the caller.  Example Data: Entrypoint, Env, WorkingDir, Labels ...
func imageData(c *cli.Context, runtime *libpod.Runtime, image string) (string, string, *inspect.ImageData, error) {
	var (
		err                error
		imageName, imageID string
	)
	// Deal with the image after all the args have been checked
	createImage := runtime.NewImage(image)
	imageName, imageID, _ = createImage.GetLocalImageName()
	if createImage.LocalName == "" {
		// The image wasnt found by the user input'd name or its fqname
		// Pull the image
		var writer io.Writer
		if !c.Bool("quiet") {
			writer = os.Stderr
		}
		createImage.Pull(writer)
	}

	createImage.LocalName = imageName
	if imageName == "" {
		imageName, err = createImage.GetFQName()
		_, imageID, _ = createImage.GetLocalImageName()
	}
	if err != nil {
		return "", "", nil, err
	}
	storageImage, err := runtime.GetImage(imageName)
	if err != nil {
		return "", "", nil, errors.Wrapf(err, "error getting storage image %q", image)
	}
	data, err := runtime.GetImageInspectInfo(*storageImage)
	if err != nil {
		return "", "", nil, errors.Wrapf(err, "error parsing image data %q", image)
	}
	return imageName, imageID, data, err
}

// Parses CLI options related to container creation into a config which can be
// parsed into an OCI runtime spec
func parseCreateOpts(c *cli.Context, runtime *libpod.Runtime, imageName string, data *inspect.ImageData) (*createConfig, error) {
	var inputCommand, command []string
	var memoryLimit, memoryReservation, memorySwap, memoryKernel int64
	var blkioWeight uint16

	imageID := data.ID

	if len(c.Args()) > 1 {
		inputCommand = c.Args()[1:]
	}

	sysctl, err := validateSysctl(c.StringSlice("sysctl"))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid value for sysctl")
	}

	groupAdd, err := stringSlicetoUint32Slice(c.StringSlice("group-add"))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid value for groups provided")
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

	// Because we cannot do a non-terminal attach, we need to set tty to true
	// if detach is not false
	// TODO Allow non-terminal attach
	tty := c.Bool("tty")
	if !c.Bool("detach") && !tty {
		tty = true
	}

	pidMode := container.PidMode(c.String("pid"))
	if !pidMode.Valid() {
		return nil, errors.Errorf("--pid %q is not valid", c.String("pid"))
	}

	if c.Bool("detach") && c.Bool("rm") {
		return nil, errors.Errorf("--rm and --detach can not be specified together")
	}
	if c.Int64("cpu-period") != 0 && c.Float64("cpus") > 0 {
		return nil, errors.Errorf("--cpu-period and --cpus cannot be set together")
	}
	if c.Int64("cpu-quota") != 0 && c.Float64("cpus") > 0 {
		return nil, errors.Errorf("--cpu-quota and --cpus cannot be set together")
	}

	utsMode := container.UTSMode(c.String("uts"))
	if !utsMode.Valid() {
		return nil, errors.Errorf("--uts %q is not valid", c.String("uts"))
	}
	ipcMode := container.IpcMode(c.String("ipc"))
	if !ipcMode.Valid() {
		return nil, errors.Errorf("--ipc %q is not valid", ipcMode)
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
		user = data.ContainerConfig.User
	}

	// STOP SIGNAL
	stopSignal := syscall.SIGTERM
	signalString := data.ContainerConfig.StopSignal
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
	for _, e := range data.ContainerConfig.Env {
		split := strings.SplitN(e, "=", 2)
		if len(split) > 1 {
			env[split[0]] = split[1]
		} else {
			env[split[0]] = ""
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
	for key, val := range data.ContainerConfig.Labels {
		if _, ok := labels[key]; !ok {
			labels[key] = val
		}
	}

	// WORKING DIRECTORY
	workDir := c.String("workdir")
	if workDir == "" {
		workDir = data.ContainerConfig.WorkingDir
	}

	// ENTRYPOINT
	// User input entrypoint takes priority over image entrypoint
	entrypoint := c.StringSlice("entrypoint")
	if len(entrypoint) == 0 {
		entrypoint = data.ContainerConfig.Entrypoint
	}

	// Build the command
	// If we have an entry point, it goes first
	if len(entrypoint) > 0 {
		command = entrypoint
	}
	if len(inputCommand) > 0 {
		// User command overrides data CMD
		command = append(command, inputCommand...)
	} else if len(data.ContainerConfig.Cmd) > 0 && !c.IsSet("entrypoint") {
		// If not user command, add CMD
		command = append(command, data.ContainerConfig.Cmd...)
	}

	if len(command) == 0 {
		return nil, errors.Errorf("No command specified on command line or as CMD or ENTRYPOINT in this image")
	}

	// EXPOSED PORTS
	portBindings, err := exposedPorts(c, data.ContainerConfig.ExposedPorts)
	if err != nil {
		return nil, err
	}

	// SHM SIze
	shmSize, err := units.FromHumanSize(c.String("shm-size"))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to translate --shm-size")
	}
	// Network
	// Both --network and --net have default values of 'bridge'
	// --net only overrides --network when --network is not explicitly
	// set and --net is.
	if c.IsSet("network") && c.IsSet("net") {
		return nil, errors.Errorf("cannot use --network and --net together.  use only --network instead")
	}
	networkMode := c.String("network")
	if !c.IsSet("network") && c.IsSet("net") {
		networkMode = c.String("net")
	}

	// Verify the additional hosts are in correct format
	for _, host := range c.StringSlice("add-host") {
		if _, err := validateExtraHost(host); err != nil {
			return nil, err
		}
	}

	// Check for . and dns-search domains
	if libpod.StringInSlice(".", c.StringSlice("dns-search")) && len(c.StringSlice("dns-search")) > 1 {
		return nil, errors.Errorf("cannot pass additional search domains when also specifying '.'")
	}

	ImageVolumes := data.ContainerConfig.Volumes

	var imageVolType = map[string]string{
		"bind":   "",
		"tmpfs":  "",
		"ignore": "",
	}
	if _, ok := imageVolType[c.String("image-volume")]; !ok {
		return nil, errors.Errorf("invalid image-volume type %q. Pick one of bind, tmpfs, or ignore", c.String("image-volume"))
	}

	config := &createConfig{
		Runtime:           runtime,
		BuiltinImgVolumes: ImageVolumes,
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
		GroupAdd:       groupAdd,
		Hostname:       c.String("hostname"),
		HostAdd:        c.StringSlice("add-host"),
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
		Network:        networkMode,
		NetworkAlias:   c.StringSlice("network-alias"),
		IpcMode:        ipcMode,
		NetMode:        container.NetworkMode(networkMode),
		UtsMode:        utsMode,
		PidMode:        pidMode,
		Pod:            c.String("pod"),
		Privileged:     c.Bool("privileged"),
		Publish:        c.StringSlice("publish"),
		PublishAll:     c.Bool("publish-all"),
		PortBindings:   portBindings,
		Quiet:          c.Bool("quiet"),
		ReadOnlyRootfs: c.Bool("read-only"),
		Resources: createResourceConfig{
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
		SigProxy:    c.Bool("sig-proxy"),
		StopSignal:  stopSignal,
		StopTimeout: c.Uint("stop-timeout"),
		Sysctl:      sysctl,
		Tmpfs:       c.StringSlice("tmpfs"),
		Tty:         tty,
		User:        user,
		Volumes:     c.StringSlice("volume"),
		WorkDir:     workDir,
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

//CreatePortBinding takes port (int) and IP (string) and creates an array of portbinding structs
func CreatePortBinding(hostPort int, hostIP string) []nat.PortBinding {
	pb := nat.PortBinding{
		HostPort: strconv.Itoa(hostPort),
	}
	pb.HostIP = hostIP
	return []nat.PortBinding{pb}
}
