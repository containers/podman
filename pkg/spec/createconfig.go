package createconfig

import (
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-connections/nat"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Type constants
const (
	bps = iota
	iops
)

// CreateResourceConfig represents resource elements in CreateConfig
// structures
type CreateResourceConfig struct {
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

// CreateConfig is a pre OCI spec structure.  It represents user input from varlink or the CLI
type CreateConfig struct {
	Annotations        map[string]string
	Args               []string
	CapAdd             []string // cap-add
	CapDrop            []string // cap-drop
	CidFile            string
	ConmonPidFile      string
	Cgroupns           string
	Cgroups            string
	CgroupParent       string            // cgroup-parent
	Command            []string          // Full command that will be used
	UserCommand        []string          // User-entered command (or image CMD)
	Detach             bool              // detach
	Devices            []string          // device
	DNSOpt             []string          //dns-opt
	DNSSearch          []string          //dns-search
	DNSServers         []string          //dns
	Entrypoint         []string          //entrypoint
	Env                map[string]string //env
	ExposedPorts       map[nat.Port]struct{}
	GroupAdd           []string // group-add
	HealthCheck        *manifest.Schema2HealthConfig
	NoHosts            bool
	HostAdd            []string //add-host
	Hostname           string   //hostname
	HTTPProxy          bool
	Init               bool   // init
	InitPath           string //init-path
	Image              string
	ImageID            string
	BuiltinImgVolumes  map[string]struct{} // volumes defined in the image config
	IDMappings         *storage.IDMappingOptions
	ImageVolumeType    string                 // how to handle the image volume, either bind, tmpfs, or ignore
	Interactive        bool                   //interactive
	IpcMode            namespaces.IpcMode     //ipc
	IP6Address         string                 //ipv6
	IPAddress          string                 //ip
	Labels             map[string]string      //label
	LinkLocalIP        []string               // link-local-ip
	LogDriver          string                 // log-driver
	LogDriverOpt       []string               // log-opt
	MacAddress         string                 //mac-address
	Name               string                 //name
	NetMode            namespaces.NetworkMode //net
	Network            string                 //network
	NetworkAlias       []string               //network-alias
	PidMode            namespaces.PidMode     //pid
	Pod                string                 //pod
	PodmanPath         string
	CgroupMode         namespaces.CgroupMode //cgroup
	PortBindings       nat.PortMap
	Privileged         bool     //privileged
	Publish            []string //publish
	PublishAll         bool     //publish-all
	Quiet              bool     //quiet
	ReadOnlyRootfs     bool     //read-only
	ReadOnlyTmpfs      bool     //read-only-tmpfs
	Resources          CreateResourceConfig
	RestartPolicy      string
	Rm                 bool              //rm
	StopSignal         syscall.Signal    // stop-signal
	StopTimeout        uint              // stop-timeout
	Sysctl             map[string]string //sysctl
	Systemd            bool
	Tmpfs              []string              // tmpfs
	Tty                bool                  //tty
	UsernsMode         namespaces.UsernsMode //userns
	User               string                //user
	UtsMode            namespaces.UTSMode    //uts
	Mounts             []spec.Mount
	MountsFlag         []string // mounts
	NamedVolumes       []*libpod.ContainerNamedVolume
	Volumes            []string //volume
	VolumesFrom        []string
	WorkDir            string   //workdir
	LabelOpts          []string //SecurityOpts
	NoNewPrivs         bool     //SecurityOpts
	ApparmorProfile    string   //SecurityOpts
	SeccompProfilePath string   //SecurityOpts
	SecurityOpts       []string
	Rootfs             string
	Syslog             bool // Whether to enable syslog on exit commands
}

func u32Ptr(i int64) *uint32     { u := uint32(i); return &u }
func fmPtr(i int64) *os.FileMode { fm := os.FileMode(i); return &fm }

// CreateBlockIO returns a LinuxBlockIO struct from a CreateConfig
func (c *CreateConfig) CreateBlockIO() (*spec.LinuxBlockIO, error) {
	return c.createBlockIO()
}

func (c *CreateConfig) createExitCommand(runtime *libpod.Runtime) ([]string, error) {
	config, err := runtime.GetConfig()
	if err != nil {
		return nil, err
	}

	// We need a cleanup process for containers in the current model.
	// But we can't assume that the caller is Podman - it could be another
	// user of the API.
	// As such, provide a way to specify a path to Podman, so we can
	// still invoke a cleanup process.
	cmd := c.PodmanPath
	if cmd == "" {
		cmd, _ = os.Executable()
	}

	command := []string{cmd,
		"--root", config.StorageConfig.GraphRoot,
		"--runroot", config.StorageConfig.RunRoot,
		"--log-level", logrus.GetLevel().String(),
		"--cgroup-manager", config.CgroupManager,
		"--tmpdir", config.TmpDir,
	}
	if config.OCIRuntime != "" {
		command = append(command, []string{"--runtime", config.OCIRuntime}...)
	}
	if config.StorageConfig.GraphDriverName != "" {
		command = append(command, []string{"--storage-driver", config.StorageConfig.GraphDriverName}...)
	}
	for _, opt := range config.StorageConfig.GraphDriverOptions {
		command = append(command, []string{"--storage-opt", opt}...)
	}
	if config.EventsLogger != "" {
		command = append(command, []string{"--events-backend", config.EventsLogger}...)
	}

	if c.Syslog {
		command = append(command, "--syslog", "true")
	}
	command = append(command, []string{"container", "cleanup"}...)

	if c.Rm {
		command = append(command, "--rm")
	}

	return command, nil
}

// GetContainerCreateOptions takes a CreateConfig and returns a slice of CtrCreateOptions
func (c *CreateConfig) getContainerCreateOptions(runtime *libpod.Runtime, pod *libpod.Pod, mounts []spec.Mount, namedVolumes []*libpod.ContainerNamedVolume) ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption
	var portBindings []ocicni.PortMapping
	var err error

	if c.Interactive {
		options = append(options, libpod.WithStdin())
	}
	if c.Systemd {
		options = append(options, libpod.WithSystemd())
	}
	if c.Name != "" {
		logrus.Debugf("setting container name %s", c.Name)
		options = append(options, libpod.WithName(c.Name))
	}
	if c.Pod != "" {
		logrus.Debugf("adding container to pod %s", c.Pod)
		options = append(options, runtime.WithPod(pod))
	}
	if c.Cgroups == "disabled" {
		options = append(options, libpod.WithNoCgroups())
	}
	if len(c.PortBindings) > 0 {
		portBindings, err = c.CreatePortBindings()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create port bindings")
		}
	}

	if len(mounts) != 0 || len(namedVolumes) != 0 {
		destinations := []string{}

		// Take all mount and named volume destinations.
		for _, mount := range mounts {
			destinations = append(destinations, mount.Destination)
		}
		for _, volume := range namedVolumes {
			destinations = append(destinations, volume.Dest)
		}

		options = append(options, libpod.WithUserVolumes(destinations))
	}

	if len(namedVolumes) != 0 {
		options = append(options, libpod.WithNamedVolumes(namedVolumes))
	}

	if len(c.UserCommand) != 0 {
		options = append(options, libpod.WithCommand(c.UserCommand))
	}

	// Add entrypoint unconditionally
	// If it's empty it's because it was explicitly set to "" or the image
	// does not have one
	options = append(options, libpod.WithEntrypoint(c.Entrypoint))

	networks := make([]string, 0)
	userNetworks := c.NetMode.UserDefined()
	if IsPod(userNetworks) {
		userNetworks = ""
	}
	if userNetworks != "" {
		for _, netName := range strings.Split(userNetworks, ",") {
			if netName == "" {
				return nil, errors.Wrapf(err, "container networks %q invalid", networks)
			}
			networks = append(networks, netName)
		}
	}

	if c.NetMode.IsNS() {
		ns := c.NetMode.NS()
		if ns == "" {
			return nil, errors.Errorf("invalid empty user-defined network namespace")
		}
		_, err := os.Stat(ns)
		if err != nil {
			return nil, err
		}
	} else if c.NetMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.NetMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.NetMode.Container())
		}
		options = append(options, libpod.WithNetNSFrom(connectedCtr))
	} else if !c.NetMode.IsHost() && !c.NetMode.IsNone() {
		hasUserns := c.UsernsMode.IsContainer() || c.UsernsMode.IsNS() || len(c.IDMappings.UIDMap) > 0 || len(c.IDMappings.GIDMap) > 0
		postConfigureNetNS := hasUserns && !c.UsernsMode.IsHost()
		options = append(options, libpod.WithNetNS(portBindings, postConfigureNetNS, string(c.NetMode), networks))
	}

	if c.CgroupMode.IsNS() {
		ns := c.CgroupMode.NS()
		if ns == "" {
			return nil, errors.Errorf("invalid empty user-defined network namespace")
		}
		_, err := os.Stat(ns)
		if err != nil {
			return nil, err
		}
	} else if c.CgroupMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.CgroupMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.CgroupMode.Container())
		}
		options = append(options, libpod.WithCgroupNSFrom(connectedCtr))
	}

	if c.UsernsMode.IsNS() {
		ns := c.UsernsMode.NS()
		if ns == "" {
			return nil, errors.Errorf("invalid empty user-defined user namespace")
		}
		_, err := os.Stat(ns)
		if err != nil {
			return nil, err
		}
		options = append(options, libpod.WithIDMappings(*c.IDMappings))
	} else if c.UsernsMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.UsernsMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.UsernsMode.Container())
		}
		options = append(options, libpod.WithUserNSFrom(connectedCtr))
	} else {
		options = append(options, libpod.WithIDMappings(*c.IDMappings))
	}

	if c.PidMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.PidMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.PidMode.Container())
		}

		options = append(options, libpod.WithPIDNSFrom(connectedCtr))
	}

	if c.IpcMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.IpcMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.IpcMode.Container())
		}

		options = append(options, libpod.WithIPCNSFrom(connectedCtr))
	}

	if IsPod(string(c.UtsMode)) {
		options = append(options, libpod.WithUTSNSFromPod(pod))
	}
	if c.UtsMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.UtsMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.UtsMode.Container())
		}

		options = append(options, libpod.WithUTSNSFrom(connectedCtr))
	}

	// TODO: MNT, USER, CGROUP
	options = append(options, libpod.WithStopSignal(c.StopSignal))
	options = append(options, libpod.WithStopTimeout(c.StopTimeout))
	if len(c.DNSSearch) > 0 {
		options = append(options, libpod.WithDNSSearch(c.DNSSearch))
	}
	if len(c.DNSServers) > 0 {
		if len(c.DNSServers) == 1 && strings.ToLower(c.DNSServers[0]) == "none" {
			options = append(options, libpod.WithUseImageResolvConf())
		} else {
			options = append(options, libpod.WithDNS(c.DNSServers))
		}
	}
	if len(c.DNSOpt) > 0 {
		options = append(options, libpod.WithDNSOption(c.DNSOpt))
	}
	if c.NoHosts {
		options = append(options, libpod.WithUseImageHosts())
	}
	if len(c.HostAdd) > 0 && !c.NoHosts {
		options = append(options, libpod.WithHosts(c.HostAdd))
	}
	logPath := getLoggingPath(c.LogDriverOpt)
	if logPath != "" {
		options = append(options, libpod.WithLogPath(logPath))
	}

	if c.LogDriver != "" {
		options = append(options, libpod.WithLogDriver(c.LogDriver))
	}

	if c.IPAddress != "" {
		ip := net.ParseIP(c.IPAddress)
		if ip == nil {
			return nil, errors.Wrapf(define.ErrInvalidArg, "cannot parse %s as IP address", c.IPAddress)
		} else if ip.To4() == nil {
			return nil, errors.Wrapf(define.ErrInvalidArg, "%s is not an IPv4 address", c.IPAddress)
		}
		options = append(options, libpod.WithStaticIP(ip))
	}

	options = append(options, libpod.WithPrivileged(c.Privileged))

	useImageVolumes := c.ImageVolumeType == TypeBind
	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(c.ImageID, c.Image, useImageVolumes))
	options = append(options, libpod.WithSecLabels(c.LabelOpts))
	options = append(options, libpod.WithConmonPidFile(c.ConmonPidFile))
	options = append(options, libpod.WithLabels(c.Labels))
	options = append(options, libpod.WithUser(c.User))
	if c.IpcMode.IsHost() {
		options = append(options, libpod.WithShmDir("/dev/shm"))

	} else if c.IpcMode.IsContainer() {
		ctr, err := runtime.LookupContainer(c.IpcMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.IpcMode.Container())
		}
		options = append(options, libpod.WithShmDir(ctr.ShmDir()))
	}
	options = append(options, libpod.WithShmSize(c.Resources.ShmSize))
	options = append(options, libpod.WithGroups(c.GroupAdd))
	if c.Rootfs != "" {
		options = append(options, libpod.WithRootFS(c.Rootfs))
	}
	// Default used if not overridden on command line

	if c.CgroupParent != "" {
		options = append(options, libpod.WithCgroupParent(c.CgroupParent))
	}

	if c.RestartPolicy != "" {
		if c.RestartPolicy == "unless-stopped" {
			return nil, errors.Wrapf(define.ErrInvalidArg, "the unless-stopped restart policy is not supported")
		}

		split := strings.Split(c.RestartPolicy, ":")
		if len(split) > 1 {
			numTries, err := strconv.Atoi(split[1])
			if err != nil {
				return nil, errors.Wrapf(err, "%s is not a valid number of retries for restart policy", split[1])
			}
			if numTries < 0 {
				return nil, errors.Wrapf(define.ErrInvalidArg, "restart policy requires a positive number of retries")
			}
			options = append(options, libpod.WithRestartRetries(uint(numTries)))
		}
		options = append(options, libpod.WithRestartPolicy(split[0]))
	}

	// Always use a cleanup process to clean up Podman after termination
	exitCmd, err := c.createExitCommand(runtime)
	if err != nil {
		return nil, err
	}
	options = append(options, libpod.WithExitCommand(exitCmd))

	if c.HealthCheck != nil {
		options = append(options, libpod.WithHealthCheck(c.HealthCheck))
		logrus.Debugf("New container has a health check")
	}
	return options, nil
}

// CreatePortBindings iterates ports mappings and exposed ports into a format CNI understands
func (c *CreateConfig) CreatePortBindings() ([]ocicni.PortMapping, error) {
	return NatToOCIPortBindings(c.PortBindings)
}

// NatToOCIPortBindings iterates a nat.portmap slice and creates []ocicni portmapping slice
func NatToOCIPortBindings(ports nat.PortMap) ([]ocicni.PortMapping, error) {
	var portBindings []ocicni.PortMapping
	for containerPb, hostPb := range ports {
		var pm ocicni.PortMapping
		pm.ContainerPort = int32(containerPb.Int())
		for _, i := range hostPb {
			var hostPort int
			var err error
			pm.HostIP = i.HostIP
			if i.HostPort == "" {
				hostPort = containerPb.Int()
			} else {
				hostPort, err = strconv.Atoi(i.HostPort)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to convert host port to integer")
				}
			}

			pm.HostPort = int32(hostPort)
			pm.Protocol = containerPb.Proto()
			portBindings = append(portBindings, pm)
		}
	}
	return portBindings, nil
}

// AddPrivilegedDevices iterates through host devices and adds all
// host devices to the spec
func (c *CreateConfig) AddPrivilegedDevices(g *generate.Generator) error {
	return c.addPrivilegedDevices(g)
}
