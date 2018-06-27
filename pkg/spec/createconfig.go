package createconfig

import (
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/runc/libcontainer/devices"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/pkg/rootless"
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
	StopSignal         syscall.Signal       // stop-signal
	StopTimeout        uint                 // stop-timeout
	Sysctl             map[string]string    //sysctl
	Tmpfs              []string             // tmpfs
	Tty                bool                 //tty
	UsernsMode         container.UsernsMode //userns
	User               string               //user
	UtsMode            container.UTSMode    //uts
	Volumes            []string             //volume
	WorkDir            string               //workdir
	MountLabel         string               //SecurityOpts
	ProcessLabel       string               //SecurityOpts
	NoNewPrivs         bool                 //SecurityOpts
	ApparmorProfile    string               //SecurityOpts
	SeccompProfilePath string               //SecurityOpts
	SecurityOpts       []string
	Rootfs             string
}

func u32Ptr(i int64) *uint32     { u := uint32(i); return &u }
func fmPtr(i int64) *os.FileMode { fm := os.FileMode(i); return &fm }

// CreateBlockIO returns a LinuxBlockIO struct from a CreateConfig
func (c *CreateConfig) CreateBlockIO() (*spec.LinuxBlockIO, error) {
	bio := &spec.LinuxBlockIO{}
	bio.Weight = &c.Resources.BlkioWeight
	if len(c.Resources.BlkioWeightDevice) > 0 {
		var lwds []spec.LinuxWeightDevice
		for _, i := range c.Resources.BlkioWeightDevice {
			wd, err := validateweightDevice(i)
			if err != nil {
				return bio, errors.Wrapf(err, "invalid values for blkio-weight-device")
			}
			wdStat, err := getStatFromPath(wd.path)
			if err != nil {
				return bio, errors.Wrapf(err, "error getting stat from path %q", wd.path)
			}
			lwd := spec.LinuxWeightDevice{
				Weight: &wd.weight,
			}
			lwd.Major = int64(unix.Major(wdStat.Rdev))
			lwd.Minor = int64(unix.Minor(wdStat.Rdev))
			lwds = append(lwds, lwd)
		}
		bio.WeightDevice = lwds
	}
	if len(c.Resources.DeviceReadBps) > 0 {
		readBps, err := makeThrottleArray(c.Resources.DeviceReadBps, bps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadBpsDevice = readBps
	}
	if len(c.Resources.DeviceWriteBps) > 0 {
		writeBpds, err := makeThrottleArray(c.Resources.DeviceWriteBps, bps)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteBpsDevice = writeBpds
	}
	if len(c.Resources.DeviceReadIOps) > 0 {
		readIOps, err := makeThrottleArray(c.Resources.DeviceReadIOps, iops)
		if err != nil {
			return bio, err
		}
		bio.ThrottleReadIOPSDevice = readIOps
	}
	if len(c.Resources.DeviceWriteIOps) > 0 {
		writeIOps, err := makeThrottleArray(c.Resources.DeviceWriteIOps, iops)
		if err != nil {
			return bio, err
		}
		bio.ThrottleWriteIOPSDevice = writeIOps
	}
	return bio, nil
}

func makeThrottleArray(throttleInput []string, rateType int) ([]spec.LinuxThrottleDevice, error) {
	var (
		ltds []spec.LinuxThrottleDevice
		t    *throttleDevice
		err  error
	)
	for _, i := range throttleInput {
		if rateType == bps {
			t, err = validateBpsDevice(i)
		} else {
			t, err = validateIOpsDevice(i)
		}
		if err != nil {
			return []spec.LinuxThrottleDevice{}, err
		}
		ltdStat, err := getStatFromPath(t.path)
		if err != nil {
			return ltds, errors.Wrapf(err, "error getting stat from path %q", t.path)
		}
		ltd := spec.LinuxThrottleDevice{
			Rate: t.rate,
		}
		ltd.Major = int64(unix.Major(ltdStat.Rdev))
		ltd.Minor = int64(unix.Minor(ltdStat.Rdev))
		ltds = append(ltds, ltd)
	}
	return ltds, nil
}

//GetVolumeMounts takes user provided input for bind mounts and creates Mount structs
func (c *CreateConfig) GetVolumeMounts(specMounts []spec.Mount) ([]spec.Mount, error) {
	var m []spec.Mount
	var options []string
	for _, i := range c.Volumes {
		// We need to handle SELinux options better here, specifically :Z
		spliti := strings.Split(i, ":")
		if len(spliti) > 2 {
			options = strings.Split(spliti[2], ",")
		}
		if libpod.MountExists(specMounts, spliti[1]) {
			continue
		}
		options = append(options, "rbind")
		var foundrw, foundro, foundz, foundZ bool
		var rootProp string
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
			Options:     []string{"rw", "noexec", "nosuid", "nodev", "tmpcopyup"},
		}
		m = append(m, mount)
	}
	return m, nil
}

//GetTmpfsMounts takes user provided input for Tmpfs mounts and creates Mount structs
func (c *CreateConfig) GetTmpfsMounts() []spec.Mount {
	var m []spec.Mount
	for _, i := range c.Tmpfs {
		// Default options if nothing passed
		options := []string{"rw", "noexec", "nosuid", "nodev", "size=65536k"}
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

// GetContainerCreateOptions takes a CreateConfig and returns a slice of CtrCreateOptions
func (c *CreateConfig) GetContainerCreateOptions() ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption
	var portBindings []ocicni.PortMapping
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

	if len(c.Command) != 0 {
		options = append(options, libpod.WithCommand(c.Command))
	}

	// Add entrypoint unconditionally
	// If it's empty it's because it was explicitly set to "" or the image
	// does not have one
	options = append(options, libpod.WithEntrypoint(c.Entrypoint))

	if rootless.IsRootless() {
		if !c.NetMode.IsHost() && !c.NetMode.IsNone() {
			options = append(options, libpod.WithNetNS(portBindings, true))
		}
	} else if c.NetMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.NetMode.ConnectedContainer())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.NetMode.ConnectedContainer())
		}
		options = append(options, libpod.WithNetNSFrom(connectedCtr))
	} else if !c.NetMode.IsHost() && !c.NetMode.IsNone() {
		postConfigureNetNS := (len(c.IDMappings.UIDMap) > 0 || len(c.IDMappings.GIDMap) > 0) && !c.UsernsMode.IsHost()
		options = append(options, libpod.WithNetNS([]ocicni.PortMapping{}, postConfigureNetNS))
		options = append(options, libpod.WithNetNS(portBindings, postConfigureNetNS))
	}

	if c.PidMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.PidMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.PidMode.Container())
		}

		options = append(options, libpod.WithPIDNSFrom(connectedCtr))
	}
	if c.IpcMode.IsContainer() {
		connectedCtr, err := c.Runtime.LookupContainer(c.IpcMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.IpcMode.Container())
		}

		options = append(options, libpod.WithIPCNSFrom(connectedCtr))
	}

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
			// CNI requires us to make both udp and tcp structs
			pm.Protocol = "udp"
			portBindings = append(portBindings, pm)
			pm.Protocol = "tcp"
			portBindings = append(portBindings, pm)
		}
	}
	return portBindings, nil
}

// AddPrivilegedDevices iterates through host devices and adds all
// host devices to the spec
func (c *CreateConfig) AddPrivilegedDevices(g *generate.Generator) error {
	hostDevices, err := devices.HostDevices()
	if err != nil {
		return err
	}
	g.ClearLinuxDevices()
	for _, d := range hostDevices {
		g.AddDevice(Device(d))
	}
	g.AddLinuxResourcesDevice(true, "", nil, nil, "rwm")
	return nil
}

func getStatFromPath(path string) (unix.Stat_t, error) {
	s := unix.Stat_t{}
	err := unix.Stat(path, &s)
	return s, err
}
