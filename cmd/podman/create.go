package main

import (
	"encoding/json"
	"fmt"
	"io"
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
	CPUs              string   // cpus
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
	Entrypoint         string            //entrypoint
	Env                map[string]string //env
	ExposedPorts       map[nat.Port]struct{}
	GroupAdd           []uint32 // group-add
	Hostname           string   //hostname
	Image              string
	ImageID            string
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
	StorageOpts        []string          //storage-opt
	Sysctl             map[string]string //sysctl
	Tmpfs              []string          // tmpfs
	Tty                bool              //tty
	User               string            //user
	UtsMode            container.UTSMode //uts
	Volumes            []string          //volume
	WorkDir            string            //workdir
	MountLabel         string            //SecurityOpts
	ProcessLabel       string            //SecurityOpts
	NoNewPrivileges    bool              //SecurityOpts
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
	createConfig, err := parseCreateOpts(c, runtime, imageName, data)
	if err != nil {
		return err
	}

	runtimeSpec, err := createConfigToOCISpec(createConfig)
	if err != nil {
		return err
	}
	options, err := createConfig.GetContainerCreateOptions()
	if err != nil {
		return errors.Wrapf(err, "unable to parse new container options")
	}
	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(createConfig.ImageID, createConfig.Image, true))
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

const seccompDefaultPath = "/etc/crio/seccomp.json"

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
			config.NoNewPrivileges = true
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
		if _, err := os.Stat(seccompDefaultPath); err != nil {
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "can't check if %q exists", seccompDefaultPath)
			}
		} else {
			config.SeccompProfilePath = seccompDefaultPath
		}
	}
	config.ProcessLabel, config.MountLabel, err = label.InitLabels(labelOpts)
	return err
}

func exposedPorts(c *cli.Context, imageExposedPorts map[string]struct{}) (map[nat.Port]struct{}, map[nat.Port][]nat.PortBinding, error) {
	// TODO Handle exposed ports from image
	// Currently ignoring imageExposedPorts

	ports, portBindings, err := nat.ParsePortSpecs(c.StringSlice("publish"))
	if err != nil {
		return nil, nil, err
	}

	for _, e := range c.StringSlice("expose") {
		// Merge in exposed ports to the map of published ports
		if strings.Contains(e, ":") {
			return nil, nil, fmt.Errorf("invalid port format for --expose: %s", e)
		}
		//support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		proto, port := nat.SplitProtoPort(e)
		//parse the start and end port and create a sequence of ports to expose
		//if expose a port, the start and end port are the same
		start, end, err := nat.ParsePortRange(port)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid range format for --expose: %s, error: %s", e, err)
		}
		for i := start; i <= end; i++ {
			p, err := nat.NewPort(proto, strconv.FormatUint(i, 10))
			if err != nil {
				return nil, nil, err
			}
			if _, exists := ports[p]; !exists {
				ports[p] = struct{}{}
			}
		}
	}
	return ports, portBindings, nil
}

// imageData pulls down the image if not stored locally and extracts the
// default container runtime data out of it. imageData returns the data
// to the caller.  Example Data: Entrypoint, Env, WorkingDir, Labels ...
func imageData(c *cli.Context, runtime *libpod.Runtime, image string) (string, string, *libpod.ImageData, error) {
	var err error
	// Deal with the image after all the args have been checked
	createImage := runtime.NewImage(image)
	createImage.LocalName, _ = createImage.GetLocalImageName()
	if createImage.LocalName == "" {
		// The image wasnt found by the user input'd name or its fqname
		// Pull the image
		var writer io.Writer
		if !c.Bool("quiet") {
			writer = os.Stdout
		}
		createImage.Pull(writer)
	}

	var imageName string
	if createImage.LocalName != "" {
		nameIsID, err := runtime.IsImageID(createImage.LocalName)
		if err != nil {
			return "", "", nil, err
		}
		if nameIsID {
			// If the input from the user is an ID, then we need to get the image
			// name for cstorage
			createImage.LocalName, err = createImage.GetNameByID()
			if err != nil {
				return "", "", nil, err
			}
		}
		imageName = createImage.LocalName
	} else {
		imageName, err = createImage.GetFQName()
	}
	if err != nil {
		return "", "", nil, err
	}
	imageID, err := createImage.GetImageID()
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
func parseCreateOpts(c *cli.Context, runtime *libpod.Runtime, imageName string, data *libpod.ImageData) (*createConfig, error) {
	//imageName, imageID, data, err := imageData(c, runtime, image)
	var command []string
	var memoryLimit, memoryReservation, memorySwap, memoryKernel int64
	var blkioWeight uint16

	imageID := data.ID

	if len(c.Args()) > 1 {
		command = c.Args()[1:]
	}

	sysctl, err := convertStringSliceToMap(c.StringSlice("sysctl"), "=")
	if err != nil {
		return nil, errors.Wrapf(err, "sysctl values must be in the form of KEY=VALUE")
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
		user = data.Config.User
	}

	// STOP SIGNAL
	stopSignal := syscall.SIGINT
	signalString := data.Config.StopSignal
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
	for _, e := range data.Config.Env {
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
	for key, val := range data.Config.Labels {
		if _, ok := labels[key]; !ok {
			labels[key] = val
		}
	}

	// WORKING DIRECTORY
	workDir := c.String("workdir")
	if workDir == "" {
		workDir = data.Config.WorkingDir
	}

	// COMMAND
	if len(command) == 0 {
		command = data.Config.Cmd
	}

	// ENTRYPOINT
	entrypoint := c.String("entrypoint")
	if entrypoint == "" {
		entrypoint = strings.Join(data.Config.Entrypoint, " ")
	}

	// EXPOSED PORTS
	ports, portBindings, err := exposedPorts(c, data.Config.ExposedPorts)
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

	config := &createConfig{
		Runtime:        runtime,
		CapAdd:         c.StringSlice("cap-add"),
		CapDrop:        c.StringSlice("cap-drop"),
		CgroupParent:   c.String("cgroup-parent"),
		Command:        command,
		Detach:         c.Bool("detach"),
		Devices:        c.StringSlice("device"),
		DNSOpt:         c.StringSlice("dns-opt"),
		DNSSearch:      c.StringSlice("dns-search"),
		DNSServers:     c.StringSlice("dns"),
		Entrypoint:     entrypoint,
		Env:            env,
		ExposedPorts:   ports,
		GroupAdd:       groupAdd,
		Hostname:       c.String("hostname"),
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
			CPUsetCPUs:        c.String("cpu-period"),
			CPUsetMems:        c.String("cpuset-mems"),
			CPUQuota:          c.Int64("cpu-quota"),
			CPURtPeriod:       c.Uint64("cpu-rt-period"),
			CPURtRuntime:      c.Int64("cpu-rt-runtime"),
			CPUs:              c.String("cpus"),
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
		StorageOpts: c.StringSlice("storage-opt"),
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
