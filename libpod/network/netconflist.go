package network

import (
	"net"
	"os"
	"path/filepath"
)

const (
	defaultIPv4Route = "0.0.0.0/0"
	defaultIPv6Route = "::/0"
)

// NcList describes a generic map
type NcList map[string]interface{}

// NewNcList creates a generic map of values with string
// keys and adds in version and network name
func NewNcList(name, version string) NcList {
	n := NcList{}
	n["cniVersion"] = version
	n["name"] = name
	return n
}

// NewHostLocalBridge creates a new LocalBridge for host-local
func NewHostLocalBridge(name string, isGateWay, isDefaultGW, ipMasq bool, ipamConf IPAMHostLocalConf) *HostLocalBridge {
	hostLocalBridge := HostLocalBridge{
		PluginType:  "bridge",
		BrName:      name,
		IPMasq:      ipMasq,
		HairpinMode: true,
		IPAM:        ipamConf,
	}
	if isGateWay {
		hostLocalBridge.IsGW = true
	}
	if isDefaultGW {
		hostLocalBridge.IsDefaultGW = true
	}
	return &hostLocalBridge
}

// NewIPAMHostLocalConf creates a new IPAMHostLocal configfuration
func NewIPAMHostLocalConf(routes []IPAMRoute, ipamRanges [][]IPAMLocalHostRangeConf) (IPAMHostLocalConf, error) {
	ipamConf := IPAMHostLocalConf{
		PluginType: "host-local",
		Routes:     routes,
		// Possible future support ? Leaving for clues
		//ResolveConf: "",
		//DataDir: ""
	}

	ipamConf.Ranges = ipamRanges
	return ipamConf, nil
}

// NewIPAMLocalHostRange create a new IPAM range
func NewIPAMLocalHostRange(subnet *net.IPNet, ipRange *net.IPNet, gw net.IP) ([]IPAMLocalHostRangeConf, error) { //nolint:interfacer
	var ranges []IPAMLocalHostRangeConf
	hostRange := IPAMLocalHostRangeConf{
		Subnet: subnet.String(),
	}
	// an user provided a range, we add it here
	if ipRange != nil && ipRange.IP != nil {
		first, err := FirstIPInSubnet(ipRange)
		if err != nil {
			return nil, err
		}
		last, err := LastIPInSubnet(ipRange)
		if err != nil {
			return nil, err
		}
		hostRange.RangeStart = first.String()
		hostRange.RangeEnd = last.String()
	}
	if gw != nil {
		hostRange.Gateway = gw.String()
	}
	ranges = append(ranges, hostRange)
	return ranges, nil
}

// NewIPAMRoute creates a new IPAM route configuration
func NewIPAMRoute(r *net.IPNet) IPAMRoute { //nolint:interfacer
	return IPAMRoute{Dest: r.String()}
}

// NewIPAMDefaultRoute creates a new IPAMDefault route of
// 0.0.0.0/0 for IPv4 or ::/0 for IPv6
func NewIPAMDefaultRoute(isIPv6 bool) (IPAMRoute, error) {
	route := defaultIPv4Route
	if isIPv6 {
		route = defaultIPv6Route
	}
	_, n, err := net.ParseCIDR(route)
	if err != nil {
		return IPAMRoute{}, err
	}
	return NewIPAMRoute(n), nil
}

// NewPortMapPlugin creates a predefined, default portmapping
// configuration
func NewPortMapPlugin() PortMapConfig {
	caps := make(map[string]bool)
	caps["portMappings"] = true
	p := PortMapConfig{
		PluginType:   "portmap",
		Capabilities: caps,
	}
	return p
}

// NewFirewallPlugin creates a generic firewall plugin
func NewFirewallPlugin() FirewallConfig {
	return FirewallConfig{
		PluginType: "firewall",
	}
}

// NewTuningPlugin creates a generic tuning section
func NewTuningPlugin() TuningConfig {
	return TuningConfig{
		PluginType: "tuning",
	}
}

// NewDNSNamePlugin creates the dnsname config with a given
// domainname
func NewDNSNamePlugin(domainName string) DNSNameConfig {
	caps := make(map[string]bool, 1)
	caps["aliases"] = true
	return DNSNameConfig{
		PluginType:   "dnsname",
		DomainName:   domainName,
		Capabilities: caps,
	}
}

// HasDNSNamePlugin looks to see if the dnsname cni plugin is present
func HasDNSNamePlugin(paths []string) bool {
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(p, "dnsname")); err == nil {
			return true
		}
	}
	return false
}

// NewMacVLANPlugin creates a macvlanconfig with a given device name
func NewMacVLANPlugin(device string) MacVLANConfig {
	i := IPAMDHCP{DHCP: "dhcp"}

	m := MacVLANConfig{
		PluginType: "macvlan",
		Master:     device,
		IPAM:       i,
	}
	return m
}
