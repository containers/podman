package inspect

import (
	"time"

	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// ContainerData holds the podman inspect data for a container
type ContainerData struct {
	*ContainerInspectData
	HostConfig *HostConfig `json:"HostConfig"`
	Config     *CtrConfig  `json:"Config"`
}

// HostConfig represents the host configuration for the container
type HostConfig struct {
	ContainerIDFile      string                      `json:"ContainerIDFile"`
	LogConfig            *LogConfig                  `json:"LogConfig"` //TODO
	NetworkMode          string                      `json:"NetworkMode"`
	PortBindings         nat.PortMap                 `json:"PortBindings"` //TODO
	AutoRemove           bool                        `json:"AutoRemove"`
	CapAdd               []string                    `json:"CapAdd"`
	CapDrop              []string                    `json:"CapDrop"`
	DNS                  []string                    `json:"DNS"`
	DNSOptions           []string                    `json:"DNSOptions"`
	DNSSearch            []string                    `json:"DNSSearch"`
	ExtraHosts           []string                    `json:"ExtraHosts"`
	GroupAdd             []uint32                    `json:"GroupAdd"`
	IpcMode              string                      `json:"IpcMode"`
	Cgroup               string                      `json:"Cgroup"`
	OomScoreAdj          *int                        `json:"OomScoreAdj"`
	PidMode              string                      `json:"PidMode"`
	Privileged           bool                        `json:"Privileged"`
	PublishAllPorts      bool                        `json:"PublishAllPorts"` //TODO
	ReadonlyRootfs       bool                        `json:"ReadonlyRootfs"`
	SecurityOpt          []string                    `json:"SecurityOpt"`
	UTSMode              string                      `json:"UTSMode"`
	UsernsMode           string                      `json:"UsernsMode"`
	ShmSize              int64                       `json:"ShmSize"`
	Runtime              string                      `json:"Runtime"`
	ConsoleSize          *specs.Box                  `json:"ConsoleSize"`
	CPUShares            *uint64                     `json:"CpuShares"`
	Memory               int64                       `json:"Memory"`
	NanoCPUs             int                         `json:"NanoCpus"`
	CgroupParent         string                      `json:"CgroupParent"`
	BlkioWeight          *uint16                     `json:"BlkioWeight"`
	BlkioWeightDevice    []specs.LinuxWeightDevice   `json:"BlkioWeightDevice"`
	BlkioDeviceReadBps   []specs.LinuxThrottleDevice `json:"BlkioDeviceReadBps"`
	BlkioDeviceWriteBps  []specs.LinuxThrottleDevice `json:"BlkioDeviceWriteBps"`
	BlkioDeviceReadIOps  []specs.LinuxThrottleDevice `json:"BlkioDeviceReadIOps"`
	BlkioDeviceWriteIOps []specs.LinuxThrottleDevice `json:"BlkioDeviceWriteIOps"`
	CPUPeriod            *uint64                     `json:"CpuPeriod"`
	CPUQuota             *int64                      `json:"CpuQuota"`
	CPURealtimePeriod    *uint64                     `json:"CpuRealtimePeriod"`
	CPURealtimeRuntime   *int64                      `json:"CpuRealtimeRuntime"`
	CPUSetCPUs           string                      `json:"CpuSetCpus"`
	CPUSetMems           string                      `json:"CpuSetMems"`
	Devices              []specs.LinuxDevice         `json:"Devices"`
	DiskQuota            int                         `json:"DiskQuota"` //check type, TODO
	KernelMemory         *int64                      `json:"KernelMemory"`
	MemoryReservation    *int64                      `json:"MemoryReservation"`
	MemorySwap           *int64                      `json:"MemorySwap"`
	MemorySwappiness     *uint64                     `json:"MemorySwappiness"`
	OomKillDisable       *bool                       `json:"OomKillDisable"`
	PidsLimit            *int64                      `json:"PidsLimit"`
	Ulimits              []string                    `json:"Ulimits"`
	CPUCount             int                         `json:"CpuCount"`
	CPUPercent           int                         `json:"CpuPercent"`
	IOMaximumIOps        int                         `json:"IOMaximumIOps"`      //check type, TODO
	IOMaximumBandwidth   int                         `json:"IOMaximumBandwidth"` //check type, TODO
	Tmpfs                []string                    `json:"Tmpfs"`
}

// CtrConfig holds information about the container configuration
type CtrConfig struct {
	Hostname     string              `json:"Hostname"`
	DomainName   string              `json:"Domainname"` //TODO
	User         specs.User          `json:"User"`
	AttachStdin  bool                `json:"AttachStdin"`  //TODO
	AttachStdout bool                `json:"AttachStdout"` //TODO
	AttachStderr bool                `json:"AttachStderr"` //TODO
	Tty          bool                `json:"Tty"`
	OpenStdin    bool                `json:"OpenStdin"`
	StdinOnce    bool                `json:"StdinOnce"` //TODO
	Env          []string            `json:"Env"`
	Cmd          []string            `json:"Cmd"`
	Image        string              `json:"Image"`
	Volumes      map[string]struct{} `json:"Volumes"`
	WorkingDir   string              `json:"WorkingDir"`
	Entrypoint   string              `json:"Entrypoint"`
	Labels       map[string]string   `json:"Labels"`
	Annotations  map[string]string   `json:"Annotations"`
	StopSignal   uint                `json:"StopSignal"`
}

// LogConfig holds the log information for a container
type LogConfig struct {
	Type   string            `json:"Type"`   // TODO
	Config map[string]string `json:"Config"` //idk type, TODO
}

// ImageData holds the inspect information of an image
type ImageData struct {
	ID              string            `json:"Id"`
	Digest          digest.Digest     `json:"Digest"`
	RepoTags        []string          `json:"RepoTags"`
	RepoDigests     []string          `json:"RepoDigests"`
	Parent          string            `json:"Parent"`
	Comment         string            `json:"Comment"`
	Created         *time.Time        `json:"Created"`
	ContainerConfig *v1.ImageConfig   `json:"ContainerConfig"`
	Version         string            `json:"Version"`
	Author          string            `json:"Author"`
	Architecture    string            `json:"Architecture"`
	Os              string            `json:"Os"`
	Size            int64             `json:"Size"`
	VirtualSize     int64             `json:"VirtualSize"`
	GraphDriver     *Data             `json:"GraphDriver"`
	RootFS          *RootFS           `json:"RootFS"`
	Labels          map[string]string `json:"Labels"`
	Annotations     map[string]string `json:"Annotations"`
	ManifestType    string            `json:"ManifestType"`
	User            string            `json:"User"`
}

// RootFS holds the root fs information of an image
type RootFS struct {
	Type   string          `json:"Type"`
	Layers []digest.Digest `json:"Layers"`
}

// Data handles the data for a storage driver
type Data struct {
	Name string            `json:"Name"`
	Data map[string]string `json:"Data"`
}

// ContainerInspectData handles the data used when inspecting a container
type ContainerInspectData struct {
	ID              string                 `json:"ID"`
	Created         time.Time              `json:"Created"`
	Path            string                 `json:"Path"`
	Args            []string               `json:"Args"`
	State           *ContainerInspectState `json:"State"`
	ImageID         string                 `json:"Image"`
	ImageName       string                 `json:"ImageName"`
	Rootfs          string                 `json:"Rootfs"`
	ResolvConfPath  string                 `json:"ResolvConfPath"`
	HostnamePath    string                 `json:"HostnamePath"`
	HostsPath       string                 `json:"HostsPath"`
	StaticDir       string                 `json:"StaticDir"`
	LogPath         string                 `json:"LogPath"`
	Name            string                 `json:"Name"`
	RestartCount    int32                  `json:"RestartCount"` //TODO
	Driver          string                 `json:"Driver"`
	MountLabel      string                 `json:"MountLabel"`
	ProcessLabel    string                 `json:"ProcessLabel"`
	AppArmorProfile string                 `json:"AppArmorProfile"`
	EffectiveCaps   []string               `json:"EffectiveCaps"`
	BoundingCaps    []string               `json:"BoundingCaps"`
	ExecIDs         []string               `json:"ExecIDs"`
	GraphDriver     *Data                  `json:"GraphDriver"`
	SizeRw          int64                  `json:"SizeRw,omitempty"`
	SizeRootFs      int64                  `json:"SizeRootFs,omitempty"`
	Mounts          []specs.Mount          `json:"Mounts"`
	Dependencies    []string               `json:"Dependencies"`
	NetworkSettings *NetworkSettings       `json:"NetworkSettings"` //TODO
	ExitCommand     []string               `json:"ExitCommand"`
	Namespace       string                 `json:"Namespace"`
}

// ContainerInspectState represents the state of a container.
type ContainerInspectState struct {
	OciVersion string    `json:"OciVersion"`
	Status     string    `json:"Status"`
	Running    bool      `json:"Running"`
	Paused     bool      `json:"Paused"`
	Restarting bool      `json:"Restarting"` // TODO
	OOMKilled  bool      `json:"OOMKilled"`
	Dead       bool      `json:"Dead"`
	Pid        int       `json:"Pid"`
	ExitCode   int32     `json:"ExitCode"`
	Error      string    `json:"Error"` // TODO
	StartedAt  time.Time `json:"StartedAt"`
	FinishedAt time.Time `json:"FinishedAt"`
}

// NetworkSettings holds information about the newtwork settings of the container
type NetworkSettings struct {
	Bridge                 string               `json:"Bridge"`
	SandboxID              string               `json:"SandboxID"`
	HairpinMode            bool                 `json:"HairpinMode"`
	LinkLocalIPv6Address   string               `json:"LinkLocalIPv6Address"`
	LinkLocalIPv6PrefixLen int                  `json:"LinkLocalIPv6PrefixLen"`
	Ports                  []ocicni.PortMapping `json:"Ports"`
	SandboxKey             string               `json:"SandboxKey"`
	SecondaryIPAddresses   []string             `json:"SecondaryIPAddresses"`
	SecondaryIPv6Addresses []string             `json:"SecondaryIPv6Addresses"`
	EndpointID             string               `json:"EndpointID"`
	Gateway                string               `json:"Gateway"`
	GlobalIPv6Address      string               `json:"GlobalIPv6Address"`
	GlobalIPv6PrefixLen    int                  `json:"GlobalIPv6PrefixLen"`
	IPAddress              string               `json:"IPAddress"`
	IPPrefixLen            int                  `json:"IPPrefixLen"`
	IPv6Gateway            string               `json:"IPv6Gateway"`
	MacAddress             string               `json:"MacAddress"`
}

// ImageResult is used for podman images for collection and output
type ImageResult struct {
	Tag          string
	Repository   string
	RepoDigests  []string
	RepoTags     []string
	ID           string
	Digest       digest.Digest
	ConfigDigest digest.Digest
	Created      time.Time
	Size         *uint64
	Labels       map[string]string
	Dangling     bool
}
