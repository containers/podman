package libpod

import (
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projectatomic/libpod/libpod/driver"
)

// ContainerInspectData handles the data used when inspecting a container
type ContainerInspectData struct {
	ID              string                 `json:"ID"`
	Created         time.Time              `json:"Created"`
	Path            string                 `json:"Path"`
	Args            []string               `json:"Args"`
	State           *ContainerInspectState `json:"State"`
	ImageID         string                 `json:"Image"`
	ImageName       string                 `json:"ImageName"`
	ResolvConfPath  string                 `json:"ResolvConfPath"`
	HostnamePath    string                 `json:"HostnamePath"` //TODO
	HostsPath       string                 `json:"HostsPath"`    //TODO
	StaticDir       string                 `json:"StaticDir"`
	LogPath         string                 `json:"LogPath"`
	Name            string                 `json:"Name"`
	RestartCount    int32                  `json:"RestartCount"` //TODO
	Driver          string                 `json:"Driver"`
	MountLabel      string                 `json:"MountLabel"`
	ProcessLabel    string                 `json:"ProcessLabel"`
	AppArmorProfile string                 `json:"AppArmorProfile"`
	ExecIDs         []string               `json:"ExecIDs"` //TODO
	GraphDriver     *driver.Data           `json:"GraphDriver"`
	SizeRw          int64                  `json:"SizeRw,omitempty"`
	SizeRootFs      int64                  `json:"SizeRootFs,omitempty"`
	Mounts          []specs.Mount          `json:"Mounts"`
	NetworkSettings *NetworkSettings       `json:"NetworkSettings"` //TODO
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
	Bridge                 string              `json:"Bridge"`
	SandboxID              string              `json:"SandboxID"`
	HairpinMode            bool                `json:"HairpinMode"`
	LinkLocalIPv6Address   string              `json:"LinkLocalIPv6Address"`
	LinkLocalIPv6PrefixLen int                 `json:"LinkLocalIPv6PrefixLen"`
	Ports                  map[string]struct{} `json:"Ports"`
	SandboxKey             string              `json:"SandboxKey"`
	SecondaryIPAddresses   string              `json:"SecondaryIPAddresses"`   //idk type
	SecondaryIPv6Addresses string              `json:"SecondaryIPv6Addresses"` //idk type
	EndpointID             string              `json:"EndpointID"`
	Gateway                string              `json:"Gateway"`
	GlobalIPv6Addresses    string              `json:"GlobalIPv6Addresses"`
	GlobalIPv6PrefixLen    int                 `json:"GlobalIPv6PrefixLen"`
	IPAddress              string              `json:"IPAddress"`
	IPPrefixLen            int                 `json:"IPPrefixLen"`
	IPv6Gateway            string              `json:"IPv6Gateway"`
	MacAddress             string              `json:"MacAddress"`
}

// ImageData holds the inspect information of an image
type ImageData struct {
	ID           string            `json:"ID"`
	Digest       digest.Digest     `json:"Digest"`
	RepoTags     []string          `json:"RepoTags"`
	RepoDigests  []string          `json:"RepoDigests"`
	Parent       string            `json:"Parent"`
	Comment      string            `json:"Comment"`
	Created      *time.Time        `json:"Created"`
	Config       *v1.ImageConfig   `json:"Config"`
	Version      string            `json:"Version"`
	Author       string            `json:"Author"`
	Architecture string            `json:"Architecture"`
	Os           string            `json:"Os"`
	Size         int64             `json:"Size"`
	VirtualSize  int64             `json:"VirtualSize"`
	GraphDriver  *driver.Data      `json:"GraphDriver"`
	RootFS       *RootFS           `json:"RootFS"`
	Labels       map[string]string `json:"Labels"`
	Annotations  map[string]string `json:"Annotations"`
}

// RootFS holds the root fs information of an image
type RootFS struct {
	Type   string          `json:"Type"`
	Layers []digest.Digest `json:"Layers"`
}
