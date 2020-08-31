package createconfig

import (
	"context"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/namespaces"
	"github.com/containers/podman/v2/pkg/seccomp"
	"github.com/containers/storage"
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
	CgroupConf        map[string]string
	CPUPeriod         uint64  // cpu-period
	CPUQuota          int64   // cpu-quota
	CPURtPeriod       uint64  // cpu-rt-period
	CPURtRuntime      int64   // cpu-rt-runtime
	CPUShares         uint64  // cpu-shares
	CPUs              float64 // cpus
	CPUsetCPUs        string
	CPUsetMems        string   // cpuset-mems
	DeviceCgroupRules []string //device-cgroup-rule
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

// PidConfig configures the pid namespace for the container
type PidConfig struct {
	PidMode namespaces.PidMode //pid
}

// IpcConfig configures the ipc namespace for the container
type IpcConfig struct {
	IpcMode namespaces.IpcMode //ipc
}

// CgroupConfig configures the cgroup namespace for the container
type CgroupConfig struct {
	Cgroups      string
	Cgroupns     string
	CgroupParent string                // cgroup-parent
	CgroupMode   namespaces.CgroupMode //cgroup
}

// UserConfig configures the user namespace for the container
type UserConfig struct {
	GroupAdd   []string // group-add
	IDMappings *storage.IDMappingOptions
	UsernsMode namespaces.UsernsMode //userns
	User       string                //user
}

// UtsConfig configures the uts namespace for the container
type UtsConfig struct {
	UtsMode  namespaces.UTSMode //uts
	NoHosts  bool
	HostAdd  []string //add-host
	Hostname string
}

// NetworkConfig configures the network namespace for the container
type NetworkConfig struct {
	DNSOpt       []string //dns-opt
	DNSSearch    []string //dns-search
	DNSServers   []string //dns
	ExposedPorts map[nat.Port]struct{}
	HTTPProxy    bool
	IP6Address   string                 //ipv6
	IPAddress    string                 //ip
	LinkLocalIP  []string               // link-local-ip
	MacAddress   string                 //mac-address
	NetMode      namespaces.NetworkMode //net
	Network      string                 //network
	NetworkAlias []string               //network-alias
	PortBindings nat.PortMap
	Publish      []string //publish
	PublishAll   bool     //publish-all
}

// SecurityConfig configures the security features for the container
type SecurityConfig struct {
	CapAdd                  []string // cap-add
	CapDrop                 []string // cap-drop
	CapRequired             []string // cap-required
	LabelOpts               []string //SecurityOpts
	NoNewPrivs              bool     //SecurityOpts
	ApparmorProfile         string   //SecurityOpts
	SeccompProfilePath      string   //SecurityOpts
	SeccompProfileFromImage string   // seccomp profile from the container image
	SeccompPolicy           seccomp.Policy
	SecurityOpts            []string
	Privileged              bool              //privileged
	ReadOnlyRootfs          bool              //read-only
	ReadOnlyTmpfs           bool              //read-only-tmpfs
	Sysctl                  map[string]string //sysctl
	ProcOpts                []string
}

// CreateConfig is a pre OCI spec structure.  It represents user input from varlink or the CLI
// swagger:model CreateConfig
type CreateConfig struct {
	Annotations       map[string]string
	Args              []string
	CidFile           string
	ConmonPidFile     string
	Command           []string          // Full command that will be used
	UserCommand       []string          // User-entered command (or image CMD)
	Detach            bool              // detach
	Devices           []string          // device
	Entrypoint        []string          //entrypoint
	Env               map[string]string //env
	HealthCheck       *manifest.Schema2HealthConfig
	Init              bool   // init
	InitPath          string //init-path
	Image             string
	ImageID           string
	RawImageName      string
	BuiltinImgVolumes map[string]struct{} // volumes defined in the image config
	ImageVolumeType   string              // how to handle the image volume, either bind, tmpfs, or ignore
	Interactive       bool                //interactive
	Labels            map[string]string   //label
	LogDriver         string              // log-driver
	LogDriverOpt      []string            // log-opt
	Name              string              //name
	PodmanPath        string
	Pod               string //pod
	Quiet             bool   //quiet
	Resources         CreateResourceConfig
	RestartPolicy     string
	Rm                bool           //rm
	Rmi               bool           //rmi
	StopSignal        syscall.Signal // stop-signal
	StopTimeout       uint           // stop-timeout
	Systemd           bool
	Tmpfs             []string // tmpfs
	Tty               bool     //tty
	Mounts            []spec.Mount
	MountsFlag        []string // mounts
	NamedVolumes      []*libpod.ContainerNamedVolume
	Volumes           []string //volume
	VolumesFrom       []string
	WorkDir           string //workdir
	Rootfs            string
	Security          SecurityConfig
	Syslog            bool // Whether to enable syslog on exit commands

	// Namespaces
	Pid     PidConfig
	Ipc     IpcConfig
	Cgroup  CgroupConfig
	User    UserConfig
	Uts     UtsConfig
	Network NetworkConfig
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
	storageConfig := runtime.StorageConfig()

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
		"--root", storageConfig.GraphRoot,
		"--runroot", storageConfig.RunRoot,
		"--log-level", logrus.GetLevel().String(),
		"--cgroup-manager", config.Engine.CgroupManager,
		"--tmpdir", config.Engine.TmpDir,
	}
	if config.Engine.OCIRuntime != "" {
		command = append(command, []string{"--runtime", config.Engine.OCIRuntime}...)
	}
	if storageConfig.GraphDriverName != "" {
		command = append(command, []string{"--storage-driver", storageConfig.GraphDriverName}...)
	}
	for _, opt := range storageConfig.GraphDriverOptions {
		command = append(command, []string{"--storage-opt", opt}...)
	}
	if config.Engine.EventsLogger != "" {
		command = append(command, []string{"--events-backend", config.Engine.EventsLogger}...)
	}

	if c.Syslog {
		command = append(command, "--syslog", "true")
	}
	command = append(command, []string{"container", "cleanup"}...)

	if c.Rm {
		command = append(command, "--rm")
	}

	if c.Rmi {
		command = append(command, "--rmi")
	}

	return command, nil
}

// GetContainerCreateOptions takes a CreateConfig and returns a slice of CtrCreateOptions
func (c *CreateConfig) getContainerCreateOptions(runtime *libpod.Runtime, pod *libpod.Pod, mounts []spec.Mount, namedVolumes []*libpod.ContainerNamedVolume) ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption
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

	// handle some spec from the InfraContainer when it's a pod
	if pod != nil && pod.HasInfraContainer() {
		InfraCtr, err := pod.InfraContainer()
		if err != nil {
			return nil, err
		}
		// handle the pod.spec.hostAliases
		options = append(options, libpod.WithHosts(InfraCtr.HostsAdd()))
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

	// Add entrypoint if it was set
	// If it's empty it's because it was explicitly set to ""
	if c.Entrypoint != nil {
		options = append(options, libpod.WithEntrypoint(c.Entrypoint))
	}

	// TODO: MNT, USER, CGROUP
	options = append(options, libpod.WithStopSignal(c.StopSignal))
	options = append(options, libpod.WithStopTimeout(c.StopTimeout))

	logPath, logTag := getLoggingOpts(c.LogDriverOpt)
	if logPath != "" {
		options = append(options, libpod.WithLogPath(logPath))
	}
	if logTag != "" {
		options = append(options, libpod.WithLogTag(logTag))
	}

	if c.LogDriver != "" {
		options = append(options, libpod.WithLogDriver(c.LogDriver))
	}

	secOpts, err := c.Security.ToCreateOptions()
	if err != nil {
		return nil, err
	}
	options = append(options, secOpts...)

	nsOpts, err := c.Cgroup.ToCreateOptions(runtime)
	if err != nil {
		return nil, err
	}
	options = append(options, nsOpts...)

	nsOpts, err = c.Ipc.ToCreateOptions(runtime)
	if err != nil {
		return nil, err
	}
	options = append(options, nsOpts...)

	nsOpts, err = c.Pid.ToCreateOptions(runtime)
	if err != nil {
		return nil, err
	}
	options = append(options, nsOpts...)

	nsOpts, err = c.Network.ToCreateOptions(runtime, &c.User)
	if err != nil {
		return nil, err
	}
	options = append(options, nsOpts...)

	nsOpts, err = c.Uts.ToCreateOptions(runtime, pod)
	if err != nil {
		return nil, err
	}
	options = append(options, nsOpts...)

	nsOpts, err = c.User.ToCreateOptions(runtime)
	if err != nil {
		return nil, err
	}
	options = append(options, nsOpts...)

	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(c.ImageID, c.Image, c.RawImageName))
	options = append(options, libpod.WithConmonPidFile(c.ConmonPidFile))
	options = append(options, libpod.WithLabels(c.Labels))
	options = append(options, libpod.WithShmSize(c.Resources.ShmSize))
	if c.Rootfs != "" {
		options = append(options, libpod.WithRootFS(c.Rootfs))
	}
	// Default used if not overridden on command line

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

// AddPrivilegedDevices iterates through host devices and adds all
// host devices to the spec
func AddPrivilegedDevices(g *generate.Generator) error {
	return addPrivilegedDevices(g)
}

func CreateContainerFromCreateConfig(ctx context.Context, r *libpod.Runtime, createConfig *CreateConfig, pod *libpod.Pod) (*libpod.Container, error) {
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
