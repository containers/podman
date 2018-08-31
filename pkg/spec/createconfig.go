package createconfig

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type mountType string

// Type constants
const (
	bps = iota
	iops
	// TypeBind is the type for mounting host dir
	TypeBind mountType = "bind"
	// TypeVolume is the type for remote storage volumes
	// TypeVolume mountType = "volume"  // re-enable upon use
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs mountType = "tmpfs"
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
	Runtime            *libpod.Runtime
	Annotations        map[string]string
	Args               []string
	CapAdd             []string // cap-add
	CapDrop            []string // cap-drop
	CidFile            string
	ConmonPidFile      string
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
	GroupAdd           []string // group-add
	HostAdd            []string //add-host
	Hostname           string   //hostname
	Image              string
	ImageID            string
	BuiltinImgVolumes  map[string]struct{} // volumes defined in the image config
	IDMappings         *storage.IDMappingOptions
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
	Pod                string                //pod
	PortBindings       nat.PortMap
	Privileged         bool     //privileged
	Publish            []string //publish
	PublishAll         bool     //publish-all
	Quiet              bool     //quiet
	ReadOnlyRootfs     bool     //read-only
	Resources          CreateResourceConfig
	Rm                 bool //rm
	ShmDir             string
	StopSignal         syscall.Signal    // stop-signal
	StopTimeout        uint              // stop-timeout
	Sysctl             map[string]string //sysctl
	Systemd            bool
	Tmpfs              []string             // tmpfs
	Tty                bool                 //tty
	UsernsMode         container.UsernsMode //userns
	User               string               //user
	UtsMode            container.UTSMode    //uts
	Volumes            []string             //volume
	VolumesFrom        []string
	WorkDir            string //workdir
	MountLabel         string //SecurityOpts
	ProcessLabel       string //SecurityOpts
	NoNewPrivs         bool   //SecurityOpts
	ApparmorProfile    string //SecurityOpts
	SeccompProfilePath string //SecurityOpts
	SecurityOpts       []string
	Rootfs             string
	LocalVolumes       []string //Keeps track of the built-in volumes of container used in the --volumes-from flag
}

func u32Ptr(i int64) *uint32     { u := uint32(i); return &u }
func fmPtr(i int64) *os.FileMode { fm := os.FileMode(i); return &fm }

// CreateBlockIO returns a LinuxBlockIO struct from a CreateConfig
func (c *CreateConfig) CreateBlockIO() (*spec.LinuxBlockIO, error) {
	return c.createBlockIO()
}

//GetVolumeMounts takes user provided input for bind mounts and creates Mount structs
func (c *CreateConfig) GetVolumeMounts(specMounts []spec.Mount) ([]spec.Mount, error) {
	var m []spec.Mount
	for _, i := range c.Volumes {
		var (
			options                          []string
			foundrw, foundro, foundz, foundZ bool
			rootProp                         string
		)

		// We need to handle SELinux options better here, specifically :Z
		spliti := strings.Split(i, ":")
		if len(spliti) > 2 {
			options = strings.Split(spliti[2], ",")
		}
		if libpod.MountExists(specMounts, spliti[1]) {
			continue
		}
		options = append(options, "rbind")
		for _, opt := range options {
			switch opt {
			case "rw":
				foundrw = true
			case "ro":
				foundro = true
			case "z":
				foundz = true
			case "Z":
				foundZ = true
			case "private", "rprivate", "slave", "rslave", "shared", "rshared":
				rootProp = opt
			}
		}
		if !foundrw && !foundro {
			options = append(options, "rw")
		}
		if foundz {
			if err := label.Relabel(spliti[0], c.MountLabel, true); err != nil {
				return nil, errors.Wrapf(err, "relabel failed %q", spliti[0])
			}
		}
		if foundZ {
			if err := label.Relabel(spliti[0], c.MountLabel, false); err != nil {
				return nil, errors.Wrapf(err, "relabel failed %q", spliti[0])
			}
		}
		if rootProp == "" {
			options = append(options, "private")
		}

		m = append(m, spec.Mount{
			Destination: spliti[1],
			Type:        string(TypeBind),
			Source:      spliti[0],
			Options:     options,
		})
	}

	// volumes from image config
	if c.ImageVolumeType != "tmpfs" {
		return m, nil
	}
	for vol := range c.BuiltinImgVolumes {
		if libpod.MountExists(specMounts, vol) {
			continue
		}
		mount := spec.Mount{
			Destination: vol,
			Type:        string(TypeTmpfs),
			Source:      string(TypeTmpfs),
			Options:     []string{"private", "rw", "noexec", "nosuid", "nodev", "tmpcopyup"},
		}
		m = append(m, mount)
	}
	return m, nil
}

// GetVolumesFrom reads the create-config artifact of the container to get volumes from
// and adds it to c.Volumes of the curent container.
func (c *CreateConfig) GetVolumesFrom() error {
	var options string
	for _, vol := range c.VolumesFrom {
		splitVol := strings.SplitN(vol, ":", 2)
		if len(splitVol) == 2 {
			options = splitVol[1]
		}
		ctr, err := c.Runtime.LookupContainer(splitVol[0])
		if err != nil {
			return errors.Wrapf(err, "error looking up container %q", splitVol[0])
		}
		var createArtifact CreateConfig
		artifact, err := ctr.GetArtifact("create-config")
		if err != nil {
			return errors.Wrapf(err, "error getting create-config artifact for %q", splitVol[0])
		}
		if err := json.Unmarshal(artifact, &createArtifact); err != nil {
			return err
		}

		for key := range createArtifact.BuiltinImgVolumes {
			c.LocalVolumes = append(c.LocalVolumes, key)
		}

		for _, i := range createArtifact.Volumes {
			// Volumes format is host-dir:ctr-dir[:options], so get the host and ctr dir
			// and add on the options given by the user to the flag.
			spliti := strings.SplitN(i, ":", 3)
			// Throw error if mounting volume from container with Z option (private label)
			// Override this by adding 'z' to options.
			if len(spliti) > 2 && strings.Contains(spliti[2], "Z") && !strings.Contains(options, "z") {
				return errors.Errorf("volume mounted with private option 'Z' in %q. Use option 'z' to mount in current container", ctr.ID())
			}
			if options == "" {
				// Mount the volumes with the default options
				c.Volumes = append(c.Volumes, createArtifact.Volumes...)
			} else {
				c.Volumes = append(c.Volumes, spliti[0]+":"+spliti[1]+":"+options)
			}
		}
	}
	return nil
}

//GetTmpfsMounts takes user provided input for Tmpfs mounts and creates Mount structs
func (c *CreateConfig) GetTmpfsMounts() []spec.Mount {
	var m []spec.Mount
	for _, i := range c.Tmpfs {
		// Default options if nothing passed
		options := []string{"private", "rw", "noexec", "nosuid", "nodev", "size=65536k"}
		spliti := strings.Split(i, ":")
		destPath := spliti[0]
		if len(spliti) > 1 {
			options = strings.Split(spliti[1], ",")
		}
		m = append(m, spec.Mount{
			Destination: destPath,
			Type:        string(TypeTmpfs),
			Options:     options,
			Source:      string(TypeTmpfs),
		})
	}
	return m
}

func createExitCommand(runtime *libpod.Runtime) []string {
	config := runtime.GetConfig()

	cmd, _ := os.Executable()
	command := []string{cmd,
		"--root", config.StorageConfig.GraphRoot,
		"--runroot", config.StorageConfig.RunRoot,
		"--log-level", logrus.GetLevel().String(),
		"--cgroup-manager", config.CgroupManager,
		"--tmpdir", config.TmpDir,
	}
	if config.StorageConfig.GraphDriverName != "" {
		command = append(command, []string{"--storage-driver", config.StorageConfig.GraphDriverName}...)
	}
	return append(command, []string{"container", "cleanup"}...)
}

// GetContainerCreateOptions takes a CreateConfig and returns a slice of CtrCreateOptions
func (c *CreateConfig) GetContainerCreateOptions(runtime *libpod.Runtime) ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption
	var portBindings []ocicni.PortMapping
	var pod *libpod.Pod
	var err error

	// Uncomment after talking to mheon about unimplemented funcs
	// options = append(options, libpod.WithLabels(c.labels))

	if c.Interactive {
		options = append(options, libpod.WithStdin())
	}
	if c.Name != "" {
		logrus.Debugf("appending name %s", c.Name)
		options = append(options, libpod.WithName(c.Name))
	}
	if c.Pod != "" {
		logrus.Debugf("adding container to pod %s", c.Pod)
		pod, err = runtime.LookupPod(c.Pod)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to add container to pod %s", c.Pod)
		}
		options = append(options, runtime.WithPod(pod))
	}

	if len(c.PortBindings) > 0 {
		portBindings, err = c.CreatePortBindings()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create port bindings")
		}
	}

	if len(c.Volumes) != 0 {
		// Volumes consist of multiple, comma-delineated fields
		// The image spec only includes one part of that, so drop the
		// others, if they are included
		volumes := make([]string, 0, len(c.Volumes))
		for _, vol := range c.Volumes {
			volumes = append(volumes, strings.SplitN(vol, ":", 2)[0])
		}

		options = append(options, libpod.WithUserVolumes(volumes))
	}

	if len(c.LocalVolumes) != 0 {
		options = append(options, libpod.WithLocalVolumes(c.LocalVolumes))
	}

	if len(c.Command) != 0 {
		options = append(options, libpod.WithCommand(c.Command))
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

	if IsNS(string(c.NetMode)) {
		// pass
	} else if c.NetMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.NetMode.ConnectedContainer())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.NetMode.ConnectedContainer())
		}
		options = append(options, libpod.WithNetNSFrom(connectedCtr))
	} else if IsPod(string(c.NetMode)) {
		options = append(options, libpod.WithNetNSFromPod(pod))
	} else if !c.NetMode.IsHost() && !c.NetMode.IsNone() {
		isRootless := rootless.IsRootless()
		postConfigureNetNS := isRootless || (len(c.IDMappings.UIDMap) > 0 || len(c.IDMappings.GIDMap) > 0) && !c.UsernsMode.IsHost()
		if isRootless && len(portBindings) > 0 {
			return nil, errors.New("port bindings are not yet supported by rootless containers")
		}
		options = append(options, libpod.WithNetNS(portBindings, postConfigureNetNS, networks))
	}

	if c.PidMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.PidMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.PidMode.Container())
		}

		options = append(options, libpod.WithPIDNSFrom(connectedCtr))
	}
	if IsPod(string(c.PidMode)) {
		options = append(options, libpod.WithPIDNSFromPod(pod))
	}

	if c.IpcMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.IpcMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.IpcMode.Container())
		}

		options = append(options, libpod.WithIPCNSFrom(connectedCtr))
	}
	if IsPod(string(c.IpcMode)) {
		options = append(options, libpod.WithIPCNSFromPod(pod))
	}

	if IsPod(string(c.UtsMode)) {
		options = append(options, libpod.WithUTSNSFromPod(pod))
	}

	// TODO: MNT, USER, CGROUP
	options = append(options, libpod.WithStopSignal(c.StopSignal))
	options = append(options, libpod.WithStopTimeout(c.StopTimeout))
	if len(c.DNSSearch) > 0 {
		options = append(options, libpod.WithDNSSearch(c.DNSSearch))
	}
	if len(c.DNSServers) > 0 {
		options = append(options, libpod.WithDNS(c.DNSServers))
	}
	if len(c.DNSOpt) > 0 {
		options = append(options, libpod.WithDNSOption(c.DNSOpt))
	}
	if len(c.HostAdd) > 0 {
		options = append(options, libpod.WithHosts(c.HostAdd))
	}
	logPath := getLoggingPath(c.LogDriverOpt)
	if logPath != "" {
		options = append(options, libpod.WithLogPath(logPath))
	}

	options = append(options, libpod.WithPrivileged(c.Privileged))

	useImageVolumes := c.ImageVolumeType == "bind"
	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(c.ImageID, c.Image, useImageVolumes))
	options = append(options, libpod.WithSELinuxLabels(c.ProcessLabel, c.MountLabel))
	options = append(options, libpod.WithConmonPidFile(c.ConmonPidFile))
	options = append(options, libpod.WithLabels(c.Labels))
	options = append(options, libpod.WithUser(c.User))
	options = append(options, libpod.WithShmDir(c.ShmDir))
	options = append(options, libpod.WithShmSize(c.Resources.ShmSize))
	options = append(options, libpod.WithGroups(c.GroupAdd))
	options = append(options, libpod.WithIDMappings(*c.IDMappings))
	if c.Rootfs != "" {
		options = append(options, libpod.WithRootFS(c.Rootfs))
	}
	// Default used if not overridden on command line

	if c.CgroupParent != "" {
		options = append(options, libpod.WithCgroupParent(c.CgroupParent))
	}
	if c.Detach {
		options = append(options, libpod.WithExitCommand(createExitCommand(runtime)))
	}

	return options, nil
}

// CreatePortBindings iterates ports mappings and exposed ports into a format CNI understands
func (c *CreateConfig) CreatePortBindings() ([]ocicni.PortMapping, error) {
	var portBindings []ocicni.PortMapping
	for containerPb, hostPb := range c.PortBindings {
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

func getStatFromPath(path string) (unix.Stat_t, error) {
	s := unix.Stat_t{}
	err := unix.Stat(path, &s)
	return s, err
}
