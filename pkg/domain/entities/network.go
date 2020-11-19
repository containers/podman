package entities

import (
	"net"

	"github.com/containernetworking/cni/libcni"
)

// NetworkListOptions describes options for listing networks in cli
type NetworkListOptions struct {
	Format string
	Quiet  bool
	Filter string
}

// NetworkListReport describes the results from listing networks
type NetworkListReport struct {
	*libcni.NetworkConfigList
}

// NetworkInspectReport describes the results from inspect networks
type NetworkInspectReport map[string]interface{}

// NetworkRmOptions describes options for removing networks
type NetworkRmOptions struct {
	Force bool
}

//NetworkRmReport describes the results of network removal
type NetworkRmReport struct {
	Name string
	Err  error
}

// NetworkCreateOptions describes options to create a network
// swagger:model NetworkCreateOptions
type NetworkCreateOptions struct {
	DisableDNS bool
	Driver     string
	Gateway    net.IP
	Internal   bool
	MacVLAN    string
	Range      net.IPNet
	Subnet     net.IPNet
	IPv6       bool
}

// NetworkCreateReport describes a created network for the cli
type NetworkCreateReport struct {
	Filename string
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
	Aliases   []string
	Container string
}
