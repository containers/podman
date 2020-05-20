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

// NetworkInspectOptions describes options for inspect networks
type NetworkInspectOptions struct {
	Format string
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
}

// NetworkCreateReport describes a created network for the cli
type NetworkCreateReport struct {
	Filename string
}
