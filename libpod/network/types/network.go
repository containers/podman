package types

import (
	"net"
	"time"
)

type ContainerNetwork interface {
	// NetworkCreate will take a partial filled Network and fill the
	// missing fields. It creates the Network and returns the full Network.
	NetworkCreate(Network) (Network, error)
	// NetworkRemove will remove the Network with the given name or ID.
	NetworkRemove(nameOrID string) error
	// NetworkList will return all known Networks. Optionally you can
	// supply a list of filter functions. Only if a network matches all
	// functions it is returned.
	NetworkList(...FilterFunc) ([]Network, error)
	// NetworkInspect will return the Network with the given name or ID.
	NetworkInspect(nameOrID string) (Network, error)

	// Setup will setup the container network namespace. It returns
	// a map of StatusBlocks, the key is the network name.
	Setup(namespacePath string, options SetupOptions) (map[string]StatusBlock, error)
	// Teardown will teardown the container network namespace.
	Teardown(namespacePath string, options TeardownOptions) error
}

// Network describes the Network attributes.
type Network struct {
	// Name of the Network.
	Name string `json:"name,omitempty"`
	// ID of the Network.
	ID string `json:"id,omitempty"`
	// Driver for this Network, e.g. bridge, macvlan...
	Driver string `json:"driver,omitempty"`
	// InterfaceName is the network interface name on the host.
	NetworkInterface string `json:"network_interface,omitempty"`
	// Created contains the timestamp when this network was created.
	// This is not guaranteed to stay exactly the same.
	Created time.Time
	// Subnets to use.
	Subnets []Subnet `json:"subnets,omitempty"`
	// IPv6Enabled if set to true an ipv6 subnet should be created for this net.
	IPv6Enabled bool `json:"ipv6_enabled"`
	// Internal is whether the Network should not have external routes
	// to public or other Networks.
	Internal bool `json:"internal"`
	// DNSEnabled is whether name resolution is active for container on
	// this Network.
	DNSEnabled bool `json:"dns_enabled"`
	// Labels is a set of key-value labels that have been applied to the
	// Network.
	Labels map[string]string `json:"labels,omitempty"`
	// Options is a set of key-value options that have been applied to
	// the Network.
	Options map[string]string `json:"options,omitempty"`
	// IPAMOptions contains options used for the ip assignment.
	IPAMOptions map[string]string `json:"ipam_options,omitempty"`
}

// IPNet is used as custom net.IPNet type to add Marshal/Unmarshal methods.
type IPNet struct {
	net.IPNet
}

// ParseCIDR parse a string to IPNet
func ParseCIDR(cidr string) (IPNet, error) {
	ip, net, err := net.ParseCIDR(cidr)
	if err != nil {
		return IPNet{}, err
	}
	// convert to 4 bytes if ipv4
	ipv4 := ip.To4()
	if ipv4 != nil {
		ip = ipv4
	}
	net.IP = ip
	return IPNet{*net}, err
}

func (n *IPNet) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

func (n *IPNet) UnmarshalText(text []byte) error {
	net, err := ParseCIDR(string(text))
	if err != nil {
		return err
	}
	*n = net
	return nil
}

type Subnet struct {
	// Subnet for this Network.
	Subnet IPNet `json:"subnet,omitempty"`
	// Gateway IP for this Network.
	Gateway net.IP `json:"gateway,omitempty"`
	// LeaseRange contains the range where IP are leased. Optional.
	LeaseRange *LeaseRange `json:"lease_range,omitempty"`
}

// LeaseRange contains the range where IP are leased.
type LeaseRange struct {
	// StartIP first IP in the subnet which should be used to assign ips.
	StartIP net.IP `json:"start_ip,omitempty"`
	// EndIP last IP in the subnet which should be used to assign ips.
	EndIP net.IP `json:"end_ip,omitempty"`
}

// StatusBlock contains the network information about a container
// connected to one Network.
type StatusBlock struct {
	// Interfaces contains the created network interface in the container.
	// The map key is the interface name.
	Interfaces map[string]NetInterface `json:"interfaces,omitempty"`
	// DNSServerIPs nameserver addresses which should be added to
	// the containers resolv.conf file.
	DNSServerIPs []net.IP `json:"dns_server_ips,omitempty"`
	// DNSSearchDomains search domains which should be added to
	// the containers resolv.conf file.
	DNSSearchDomains []string `json:"dns_search_domains,omitempty"`
}

// NetInterface contains the settings for a given network interface.
type NetInterface struct {
	// Networks list of assigned subnets with their gateway.
	Networks []NetAddress `json:"networks,omitempty"`
	// MacAddress for this Interface.
	MacAddress net.HardwareAddr `json:"mac_address,omitempty"`
}

// NetAddress contains the subnet and gatway.
type NetAddress struct {
	// Subnet of this NetAddress. Note that the subnet contains the
	// actual ip of the net interface and not the network address.
	Subnet IPNet `json:"subnet,omitempty"`
	// Gateway for the Subnet. This can be nil if there is no gateway, e.g. internal network.
	Gateway net.IP `json:"gateway,omitempty"`
}

// PerNetworkOptions are options which should be set on a per network basis.
type PerNetworkOptions struct {
	// StaticIPv4 for this container. Optional.
	StaticIPs []net.IP `json:"static_ips,omitempty"`
	// Aliases contains a list of names which the dns server should resolve
	// to this container. Can only be set when DNSEnabled is true on the Network.
	// Optional.
	Aliases []string `json:"aliases,omitempty"`
	// StaticMac for this container. Optional.
	StaticMAC net.HardwareAddr `json:"static_mac,omitempty"`
	// InterfaceName for this container. Required.
	InterfaceName string `json:"interface_name,omitempty"`
}

// NetworkOptions for a given container.
type NetworkOptions struct {
	// ContainerID is the container id, used for iptables comments and ipam allocation.
	ContainerID string `json:"container_id,omitempty"`
	// ContainerName is the container name, used as dns name.
	ContainerName string `json:"container_name,omitempty"`
	// PortMappings contains the port mappings for this container
	PortMappings []PortMapping `json:"port_mappings,omitempty"`
	// Networks contains all networks with the PerNetworkOptions.
	// The map should contain at least one element.
	Networks map[string]PerNetworkOptions `json:"networks,omitempty"`
}

// PortMapping is one or more ports that will be mapped into the container.
type PortMapping struct {
	// HostIP is the IP that we will bind to on the host.
	// If unset, assumed to be 0.0.0.0 (all interfaces).
	HostIP string `json:"host_ip,omitempty"`
	// ContainerPort is the port number that will be exposed from the
	// container.
	// Mandatory.
	ContainerPort uint16 `json:"container_port"`
	// HostPort is the port number that will be forwarded from the host into
	// the container.
	// If omitted, a random port on the host (guaranteed to be over 1024)
	// will be assigned.
	HostPort uint16 `json:"host_port,omitempty"`
	// Range is the number of ports that will be forwarded, starting at
	// HostPort and ContainerPort and counting up.
	// This is 1-indexed, so 1 is assumed to be a single port (only the
	// Hostport:Containerport mapping will be added), 2 is two ports (both
	// Hostport:Containerport and Hostport+1:Containerport+1), etc.
	// If unset, assumed to be 1 (a single port).
	// Both hostport + range and containerport + range must be less than
	// 65536.
	Range uint16 `json:"range,omitempty"`
	// Protocol is the protocol forward.
	// Must be either "tcp", "udp", and "sctp", or some combination of these
	// separated by commas.
	// If unset, assumed to be TCP.
	Protocol string `json:"protocol,omitempty"`
}

type SetupOptions struct {
	NetworkOptions
}

type TeardownOptions struct {
	NetworkOptions
}

// FilterFunc can be passed to NetworkList to filter the networks.
type FilterFunc func(Network) bool
