package entities

import (
	"net"

	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/storage/pkg/archive"
	"github.com/cri-o/ocicni/pkg/ocicni"
)

type Container struct {
	IdOrNamed
}

type Volume struct {
	Identifier
}

type Report struct {
	Id  []string
	Err map[string]error
}

type PodDeleteReport struct{ Report }
type PodPruneOptions struct{}

type PodPruneReport struct{ Report }
type VolumeDeleteOptions struct{}
type VolumeDeleteReport struct{ Report }

// NetOptions reflect the shared network options between
// pods and containers
type NetOptions struct {
	AddHosts     []string
	CNINetworks  []string
	DNSHost      bool
	DNSOptions   []string
	DNSSearch    []string
	DNSServers   []net.IP
	Network      specgen.Namespace
	NoHosts      bool
	PublishPorts []ocicni.PortMapping
	StaticIP     *net.IP
	StaticMAC    *net.HardwareAddr
}

// All CLI inspect commands and inspect sub-commands use the same options
type InspectOptions struct {
	Format string `json:",omitempty"`
	Latest bool   `json:",omitempty"`
	Size   bool   `json:",omitempty"`
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
