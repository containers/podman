package network

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/pkg/network"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
)

const (
	defaultIPv4Route = "0.0.0.0/0"
	defaultIPv6Route = "::/0"
)

// NcList describes a generic map
type NcList map[string]interface{}

// NcArgs describes the cni args field
type NcArgs map[string]NcLabels

// NcLabels describes the label map
type NcLabels map[string]string

// PodmanLabelKey key used to store the podman network label in a cni config
const PodmanLabelKey = "podman_labels"

// NewNcList creates a generic map of values with string
// keys and adds in version and network name
func NewNcList(name, version string, labels NcLabels) NcList {
	n := NcList{}
	n["cniVersion"] = version
	n["name"] = name
	if len(labels) > 0 {
		n["args"] = NcArgs{PodmanLabelKey: labels}
	}
	return n
}

// NewHostLocalBridge creates a new LocalBridge for host-local
func NewHostLocalBridge(name string, isGateWay, isDefaultGW, ipMasq bool, mtu int, vlan int, ipamConf IPAMConfig) *HostLocalBridge {
	hostLocalBridge := HostLocalBridge{
		PluginType:  "bridge",
		BrName:      name,
		IPMasq:      ipMasq,
		MTU:         mtu,
		HairpinMode: true,
		Vlan:        vlan,
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

// NewIPAMHostLocalConf creates a new IPAMHostLocal configuration
func NewIPAMHostLocalConf(routes []IPAMRoute, ipamRanges [][]IPAMLocalHostRangeConf) (IPAMConfig, error) {
	ipamConf := IPAMConfig{
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
	} else {
		// Add first ip in subnet as gateway. It is not required
		// by cni but should be included because of network inspect.
		hostRange.Gateway = CalcGatewayIP(subnet).String()
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
func NewMacVLANPlugin(device string, gateway net.IP, ipRange *net.IPNet, subnet *net.IPNet, mtu int) (MacVLANConfig, error) {
	i := IPAMConfig{PluginType: "dhcp"}
	if gateway != nil ||
		(ipRange != nil && ipRange.IP != nil && ipRange.Mask != nil) ||
		(subnet != nil && subnet.IP != nil && subnet.Mask != nil) {
		ipam, err := NewIPAMLocalHostRange(subnet, ipRange, gateway)
		if err != nil {
			return MacVLANConfig{}, err
		}
		ranges := make([][]IPAMLocalHostRangeConf, 0)
		ranges = append(ranges, ipam)
		i.Ranges = ranges
		route, err := NewIPAMDefaultRoute(IsIPv6(subnet.IP))
		if err != nil {
			return MacVLANConfig{}, err
		}
		i.Routes = []IPAMRoute{route}
		i.PluginType = "host-local"
	}

	m := MacVLANConfig{
		PluginType: "macvlan",
		IPAM:       i,
	}
	if mtu > 0 {
		m.MTU = mtu
	}
	// CNI is supposed to use the default route if a
	// parent device is not provided
	if len(device) > 0 {
		m.Master = device
	}
	return m, nil
}

// IfPassesFilter filters NetworkListReport and returns true if the filter match the given config
func IfPassesFilter(netconf *libcni.NetworkConfigList, filters map[string][]string) (bool, error) {
	result := true
	for key, filterValues := range filters {
		result = false
		switch strings.ToLower(key) {
		case "name":
			// matches one name, regex allowed
			result = util.StringMatchRegexSlice(netconf.Name, filterValues)

		case "plugin":
			// match one plugin
			plugins := network.GetCNIPlugins(netconf)
			for _, val := range filterValues {
				if strings.Contains(plugins, val) {
					result = true
					break
				}
			}

		case "label":
			// matches all labels
			result = util.MatchLabelFilters(filterValues, GetNetworkLabels(netconf))

		case "driver":
			// matches only for the DefaultNetworkDriver
			for _, filterValue := range filterValues {
				plugins := network.GetCNIPlugins(netconf)
				if filterValue == DefaultNetworkDriver &&
					strings.Contains(plugins, DefaultNetworkDriver) {
					result = true
				}
			}

		case "id":
			// matches part of one id
			for _, filterValue := range filterValues {
				if strings.Contains(network.GetNetworkID(netconf.Name), filterValue) {
					result = true
					break
				}
			}

		// TODO: add dangling filter

		default:
			return false, errors.Errorf("invalid filter %q", key)
		}
	}
	return result, nil
}

// IfPassesPruneFilter filters NetworkListReport and returns true if the prune filter match the given config
func IfPassesPruneFilter(config *config.Config, netconf *libcni.NetworkConfigList, f map[string][]string) (bool, error) {
	for key, filterValues := range f {
		switch strings.ToLower(key) {
		case "label":
			return util.MatchLabelFilters(filterValues, GetNetworkLabels(netconf)), nil
		case "until":
			until, err := util.ComputeUntilTimestamp(filterValues)
			if err != nil {
				return false, err
			}
			created, err := getCreatedTimestamp(config, netconf)
			if err != nil {
				return false, err
			}
			if created.Before(until) {
				return true, nil
			}
		default:
			return false, errors.Errorf("invalid filter %q", key)
		}
	}
	return false, nil
}

func getCreatedTimestamp(config *config.Config, netconf *libcni.NetworkConfigList) (*time.Time, error) {
	networkConfigPath, err := GetCNIConfigPathByNameOrID(config, netconf.Name)
	if err != nil {
		return nil, err
	}
	f, err := os.Stat(networkConfigPath)
	if err != nil {
		return nil, err
	}
	stat := f.Sys().(*syscall.Stat_t)
	created := time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)) // nolint: unconvert
	return &created, nil
}

func NewPodmanMachinePlugin() PodmanMachineConfig {
	caps := make(map[string]bool, 1)
	caps["portMappings"] = true
	return PodmanMachineConfig{
		PluginType:   "podman-machine",
		Capabilities: caps,
	}
}
