package libpod

import (
	"time"

	"github.com/containers/libpod/libpod/driver"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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
	GraphDriver     *driver.Data           `json:"GraphDriver"`
	SizeRw          int64                  `json:"SizeRw,omitempty"`
	SizeRootFs      int64                  `json:"SizeRootFs,omitempty"`
	Mounts          []specs.Mount          `json:"Mounts"`
	Dependencies    []string               `json:"Dependencies"`
	NetworkSettings *NetworkSettings       `json:"NetworkSettings"` //TODO
	ExitCommand     []string               `json:"ExitCommand"`
	Namespace       string                 `json:"Namespace"`
	IsInfra         bool                   `json:"IsInfra"`
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
	Bridge                 string        `json:"Bridge"`
	SandboxID              string        `json:"SandboxID"`
	HairpinMode            bool          `json:"HairpinMode"`
	LinkLocalIPv6Address   string        `json:"LinkLocalIPv6Address"`
	LinkLocalIPv6PrefixLen int           `json:"LinkLocalIPv6PrefixLen"`
	Ports                  []PortMapping `json:"Ports"`
	SandboxKey             string        `json:"SandboxKey"`
	SecondaryIPAddresses   []string      `json:"SecondaryIPAddresses"`
	SecondaryIPv6Addresses []string      `json:"SecondaryIPv6Addresses"`
	EndpointID             string        `json:"EndpointID"`
	Gateway                string        `json:"Gateway"`
	GlobalIPv6Address      string        `json:"GlobalIPv6Address"`
	GlobalIPv6PrefixLen    int           `json:"GlobalIPv6PrefixLen"`
	IPAddress              string        `json:"IPAddress"`
	IPPrefixLen            int           `json:"IPPrefixLen"`
	IPv6Gateway            string        `json:"IPv6Gateway"`
	MacAddress             string        `json:"MacAddress"`
}
