package network

import (
	"net"
)

//go:generate go run ../generator/generator.go CreateOptions
// CreateOptions are optional options for creating networks
type CreateOptions struct {
	// DisableDNS turns off use of DNSMasq for name resolution
	// on the network
	DisableDNS *bool
	// Driver is the name of network driver
	Driver *string
	// Gateway of the network
	Gateway *net.IP
	// Internal turns off communication outside the networking
	// being created
	Internal *bool
	// Labels are metadata that can be associated with the network
	Labels map[string]string
	// MacVLAN is the name of the macvlan network to associate with
	MacVLAN *string
	// Range is the CIDR description of leasable IP addresses
	IPRange *net.IPNet `scheme:"range"`
	// Subnet to use
	Subnet *net.IPNet
	// IPv6 means the network is ipv6 capable
	IPv6 *bool
	// Options are a mapping of driver options and values.
	Options map[string]string
	// Name of the network
	Name *string
}

//go:generate go run ../generator/generator.go InspectOptions
// InspectOptions are optional options for inspecting networks
type InspectOptions struct {
}

//go:generate go run ../generator/generator.go RemoveOptions
// RemoveOptions are optional options for inspecting networks
type RemoveOptions struct {
	// Force removes the network even if it is being used
	Force   *bool
	Timeout *uint
}

//go:generate go run ../generator/generator.go ListOptions
// ListOptions are optional options for listing networks
type ListOptions struct {
	// Filters are applied to the list of networks to be more
	// specific on the output
	Filters map[string][]string
}

//go:generate go run ../generator/generator.go DisconnectOptions
// DisconnectOptions are optional options for disconnecting
// containers from a network
type DisconnectOptions struct {
	// Force indicates to remove the container from
	// the network forcibly
	Force *bool
}

//go:generate go run ../generator/generator.go ExistsOptions
// ExistsOptions are optional options for checking
// if a network exists
type ExistsOptions struct {
}

//go:generate go run ../generator/generator.go PruneOptions
// PruneOptions are optional options for removing unused
// CNI networks
type PruneOptions struct {
	// Filters are applied to the prune of networks to be more
	// specific on choosing
	Filters map[string][]string
}
