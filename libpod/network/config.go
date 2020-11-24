package network

import (
	"encoding/json"
	"net"

	"github.com/containers/storage/pkg/lockfile"
)

// TODO once the containers.conf file stuff is worked out, this should be modified
// to honor defines in the containers.conf as well as overrides?

const (
	// CNIConfigDir is the path where CNI config files exist
	CNIConfigDir = "/etc/cni/net.d"
	// CNIDeviceName is the default network device name and in
	// reality should have an int appended to it (cni-podman4)
	CNIDeviceName = "cni-podman"
	// DefaultPodmanDomainName is used for the dnsname plugin to define
	// a localized domain name for a created network
	DefaultPodmanDomainName = "dns.podman"
	// LockFileName is used for obtaining a lock and is appended
	// to libpod's tmpdir in practice
	LockFileName = "cni.lock"
)

// CNILock is for preventing name collision and
// unpredictable results when doing some CNI operations.
type CNILock struct {
	lockfile.Locker
}

// GetDefaultPodmanNetwork outputs the default network for podman
func GetDefaultPodmanNetwork() (*net.IPNet, error) {
	_, n, err := net.ParseCIDR("10.88.1.0/24")
	return n, err
}

// CNIPlugins is a way of marshalling a CNI network configuration to disk
type CNIPlugins interface {
	Bytes() ([]byte, error)
}

// HostLocalBridge describes a configuration for a bridge plugin
// https://github.com/containernetworking/plugins/tree/master/plugins/main/bridge#network-configuration-reference
type HostLocalBridge struct {
	PluginType   string            `json:"type"`
	BrName       string            `json:"bridge,omitempty"`
	IsGW         bool              `json:"isGateway"`
	IsDefaultGW  bool              `json:"isDefaultGateway,omitempty"`
	ForceAddress bool              `json:"forceAddress,omitempty"`
	IPMasq       bool              `json:"ipMasq,omitempty"`
	MTU          int               `json:"mtu,omitempty"`
	HairpinMode  bool              `json:"hairpinMode,omitempty"`
	PromiscMode  bool              `json:"promiscMode,omitempty"`
	Vlan         int               `json:"vlan,omitempty"`
	IPAM         IPAMHostLocalConf `json:"ipam"`
}

// Bytes outputs []byte
func (h *HostLocalBridge) Bytes() ([]byte, error) {
	return json.MarshalIndent(h, "", "\t")
}

// IPAMHostLocalConf describes an IPAM configuration
// https://github.com/containernetworking/plugins/tree/master/plugins/ipam/host-local#network-configuration-reference
type IPAMHostLocalConf struct {
	PluginType  string                     `json:"type"`
	Routes      []IPAMRoute                `json:"routes,omitempty"`
	ResolveConf string                     `json:"resolveConf,omitempty"`
	DataDir     string                     `json:"dataDir,omitempty"`
	Ranges      [][]IPAMLocalHostRangeConf `json:"ranges,omitempty"`
}

// IPAMLocalHostRangeConf describes the new style IPAM ranges
type IPAMLocalHostRangeConf struct {
	Subnet     string `json:"subnet"`
	RangeStart string `json:"rangeStart,omitempty"`
	RangeEnd   string `json:"rangeEnd,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
}

// Bytes outputs the configuration as []byte
func (i IPAMHostLocalConf) Bytes() ([]byte, error) {
	return json.MarshalIndent(i, "", "\t")
}

// IPAMRoute describes a route in an ipam config
type IPAMRoute struct {
	Dest string `json:"dst"`
}

// PortMapConfig describes the default portmapping config
type PortMapConfig struct {
	PluginType   string          `json:"type"`
	Capabilities map[string]bool `json:"capabilities"`
}

// Bytes outputs the configuration as []byte
func (p PortMapConfig) Bytes() ([]byte, error) {
	return json.MarshalIndent(p, "", "\t")
}

// IPAMDHCP describes the ipamdhcp config
type IPAMDHCP struct {
	DHCP string `json:"type"`
}

// MacVLANConfig describes the macvlan config
type MacVLANConfig struct {
	PluginType string   `json:"type"`
	Master     string   `json:"master"`
	IPAM       IPAMDHCP `json:"ipam"`
}

// Bytes outputs the configuration as []byte
func (p MacVLANConfig) Bytes() ([]byte, error) {
	return json.MarshalIndent(p, "", "\t")
}

// FirewallConfig describes the firewall plugin
type FirewallConfig struct {
	PluginType string `json:"type"`
	Backend    string `json:"backend"`
}

// Bytes outputs the configuration as []byte
func (f FirewallConfig) Bytes() ([]byte, error) {
	return json.MarshalIndent(f, "", "\t")
}

// TuningConfig describes the tuning plugin
type TuningConfig struct {
	PluginType string `json:"type"`
}

// Bytes outputs the configuration as []byte
func (f TuningConfig) Bytes() ([]byte, error) {
	return json.MarshalIndent(f, "", "\t")
}

// DNSNameConfig describes the dns container name resolution plugin config
type DNSNameConfig struct {
	PluginType   string          `json:"type"`
	DomainName   string          `json:"domainName"`
	Capabilities map[string]bool `json:"capabilities"`
}

// Bytes outputs the configuration as []byte
func (d DNSNameConfig) Bytes() ([]byte, error) {
	return json.MarshalIndent(d, "", "\t")
}
