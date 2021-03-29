package entities

import (
	"net"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/podman/v3/libpod/events"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/containers/storage/pkg/archive"
)

type Container struct {
	IDOrNamed
}

type Volume struct {
	Identifier
}

type Report struct {
	Id  []string //nolint
	Err map[string]error
}

type PodDeleteReport struct{ Report }

type VolumeDeleteOptions struct{}
type VolumeDeleteReport struct{ Report }

// NetOptions reflect the shared network options between
// pods and containers
type NetOptions struct {
	AddHosts           []string
	Aliases            []string
	CNINetworks        []string
	UseImageResolvConf bool
	DNSOptions         []string
	DNSSearch          []string
	DNSServers         []net.IP
	Network            specgen.Namespace
	NoHosts            bool
	PublishPorts       []specgen.PortMapping
	StaticIP           *net.IP
	StaticMAC          *net.HardwareAddr
	// NetworkOptions are additional options for each network
	NetworkOptions map[string][]string
}

// All CLI inspect commands and inspect sub-commands use the same options
type InspectOptions struct {
	// Format - change the output to JSON or a Go template.
	Format string `json:",omitempty"`
	// Latest - inspect the latest container Podman is aware of.
	Latest bool `json:",omitempty"`
	// Size (containers only) - display total file size.
	Size bool `json:",omitempty"`
	// Type -- return JSON for specified type.
	Type string `json:",omitempty"`
	// All -- inspect all
	All bool `json:",omitempty"`
}

// All API and CLI diff commands and diff sub-commands use the same options
type DiffOptions struct {
	Format  string `json:",omitempty"` // CLI only
	Latest  bool   `json:",omitempty"` // API and CLI, only supported by containers
	Archive bool   `json:",omitempty"` // CLI only
}

// DiffReport provides changes for object
type DiffReport struct {
	Changes []archive.Change
}

type EventsOptions struct {
	FromStart bool
	EventChan chan *events.Event
	Filter    []string
	Stream    bool
	Since     string
	Until     string
}

// ContainerCreateResponse is the response struct for creating a container
type ContainerCreateResponse struct {
	// ID of the container created
	ID string `json:"Id"`
	// Warnings during container creation
	Warnings []string `json:"Warnings"`
}

// BuildOptions describe the options for building container images.
type BuildOptions struct {
	buildahDefine.BuildOptions
}

// BuildReport is the image-build report.
type BuildReport struct {
	// ID of the image.
	ID string
}
