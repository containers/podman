package entities

import (
	"net"

	"github.com/containers/common/libnetwork/types"
)

// NetworkListOptions describes options for listing networks in cli
type NetworkListOptions struct {
	Format  string
	Quiet   bool
	Filters map[string][]string
}

// NetworkReloadOptions describes options for reloading container network
// configuration.
type NetworkReloadOptions struct {
	All    bool
	Latest bool
}

// NetworkReloadReport describes the results of reloading a container network.
type NetworkReloadReport struct {
	//nolint:stylecheck,revive
	Id  string
	Err error
}

// NetworkRmOptions describes options for removing networks
type NetworkRmOptions struct {
	Force   bool
	Timeout *uint
}

// NetworkRmReport describes the results of network removal
type NetworkRmReport struct {
	Name string
	Err  error
}

// NetworkCreateOptions describes options to create a network
type NetworkCreateOptions struct {
	DisableDNS bool
	Driver     string
	Gateways   []net.IP
	Internal   bool
	Labels     map[string]string
	MacVLAN    string
	Ranges     []string
	Subnets    []string
	IPv6       bool
	// Mapping of driver options and values.
	Options map[string]string
}

// NetworkCreateReport describes a created network for the cli
type NetworkCreateReport struct {
	Name string
}

// NetworkDisconnectOptions describes options for disconnecting
// containers from networks
type NetworkDisconnectOptions struct {
	Container string
	Force     bool
}

// NetworkConnectOptions describes options for connecting
// a container to a network
type NetworkConnectOptions struct {
	Container string `json:"container"`
	types.PerNetworkOptions
}

// NetworkPruneReport containers the name of network and an error
// associated in its pruning (removal)
// swagger:model NetworkPruneReport
type NetworkPruneReport struct {
	Name  string
	Error error
}

// NetworkPruneOptions describes options for pruning unused networks
type NetworkPruneOptions struct {
	Filters map[string][]string
}
