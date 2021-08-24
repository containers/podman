// +build linux

package cni

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/libpod/network/util"
	pkgutil "github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func createNetworkFromCNIConfigList(conf *libcni.NetworkConfigList, confPath string) (*types.Network, error) {
	network := types.Network{
		Name:        conf.Name,
		ID:          getNetworkIDFromName(conf.Name),
		Labels:      map[string]string{},
		Options:     map[string]string{},
		IPAMOptions: map[string]string{},
	}

	cniJSON := make(map[string]interface{})
	err := json.Unmarshal(conf.Bytes, &cniJSON)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal network config %s", conf.Name)
	}
	if args, ok := cniJSON["args"]; ok {
		if key, ok := args.(map[string]interface{}); ok {
			// read network labels and options from the conf file
			network.Labels = getNetworkArgsFromConfList(key, podmanLabelKey)
			network.Options = getNetworkArgsFromConfList(key, podmanOptionsKey)
		}
	}

	f, err := os.Stat(confPath)
	if err != nil {
		return nil, err
	}
	stat := f.Sys().(*syscall.Stat_t)
	network.Created = time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))

	firstPlugin := conf.Plugins[0]
	network.Driver = firstPlugin.Network.Type

	switch firstPlugin.Network.Type {
	case types.BridgeNetworkDriver:
		var bridge hostLocalBridge
		err := json.Unmarshal(firstPlugin.Bytes, &bridge)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal the bridge plugin config in %s", confPath)
		}
		network.NetworkInterface = bridge.BrName

		// if isGateway is false we have an internal network
		if !bridge.IsGW {
			network.Internal = true
		}

		// set network options
		if bridge.MTU != 0 {
			network.Options["mtu"] = strconv.Itoa(bridge.MTU)
		}
		if bridge.Vlan != 0 {
			network.Options["vlan"] = strconv.Itoa(bridge.Vlan)
		}

		err = convertIPAMConfToNetwork(&network, bridge.IPAM, confPath)
		if err != nil {
			return nil, err
		}

	case types.MacVLANNetworkDriver:
		var macvlan macVLANConfig
		err := json.Unmarshal(firstPlugin.Bytes, &macvlan)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal the macvlan plugin config in %s", confPath)
		}
		network.NetworkInterface = macvlan.Master

		// set network options
		if macvlan.MTU != 0 {
			network.Options["mtu"] = strconv.Itoa(macvlan.MTU)
		}

		err = convertIPAMConfToNetwork(&network, macvlan.IPAM, confPath)
		if err != nil {
			return nil, err
		}

	default:
		// A warning would be good but users would get this warning everytime so keep this at info level.
		logrus.Infof("unsupported CNI config type %s in %s, this network can still be used but inspect or list cannot show all information",
			firstPlugin.Network.Type, confPath)
	}

	// check if the dnsname plugin is configured
	network.DNSEnabled = findPluginByName(conf.Plugins, "dnsname")

	return &network, nil
}

func findPluginByName(plugins []*libcni.NetworkConfig, name string) bool {
	for _, plugin := range plugins {
		if plugin.Network.Type == name {
			return true
		}
	}
	return false
}

// convertIPAMConfToNetwork converts A cni IPAMConfig to libpod network subnets.
// It returns an array of subnets and an extra bool if dhcp is configured.
func convertIPAMConfToNetwork(network *types.Network, ipam ipamConfig, confPath string) error {
	if ipam.PluginType == types.DHCPIPAMDriver {
		network.IPAMOptions["driver"] = types.DHCPIPAMDriver
		return nil
	}

	if ipam.PluginType != types.HostLocalIPAMDriver {
		return errors.Errorf("unsupported ipam plugin %s in %s", ipam.PluginType, confPath)
	}

	network.IPAMOptions["driver"] = types.HostLocalIPAMDriver
	for _, r := range ipam.Ranges {
		for _, ipam := range r {
			s := types.Subnet{}

			// Do not use types.ParseCIDR() because we want the ip to be
			// the network address and not a random ip in the sub.
			_, sub, err := net.ParseCIDR(ipam.Subnet)
			if err != nil {
				return err
			}
			s.Subnet = types.IPNet{IPNet: *sub}

			// gateway
			var gateway net.IP
			if ipam.Gateway != "" {
				gateway = net.ParseIP(ipam.Gateway)
				if gateway == nil {
					return errors.Errorf("failed to parse gateway ip %s", ipam.Gateway)
				}
				// convert to 4 byte if ipv4
				ipv4 := gateway.To4()
				if ipv4 != nil {
					gateway = ipv4
				}
			} else if !network.Internal {
				// only add a gateway address if the network is not internal
				gateway, err = util.FirstIPInSubnet(sub)
				if err != nil {
					return errors.Errorf("failed to get first ip in subnet %s", sub.String())
				}
			}
			s.Gateway = gateway

			var rangeStart net.IP
			var rangeEnd net.IP
			if ipam.RangeStart != "" {
				rangeStart = net.ParseIP(ipam.RangeStart)
				if rangeStart == nil {
					return errors.Errorf("failed to parse range start ip %s", ipam.RangeStart)
				}
			}
			if ipam.RangeEnd != "" {
				rangeEnd = net.ParseIP(ipam.RangeEnd)
				if rangeEnd == nil {
					return errors.Errorf("failed to parse range end ip %s", ipam.RangeEnd)
				}
			}
			if rangeStart != nil || rangeEnd != nil {
				s.LeaseRange = &types.LeaseRange{}
				s.LeaseRange.StartIP = rangeStart
				s.LeaseRange.EndIP = rangeEnd
			}
			network.Subnets = append(network.Subnets, s)
		}
	}
	return nil
}

// getNetworkArgsFromConfList returns the map of args in a conflist, argType should be labels or options
func getNetworkArgsFromConfList(args map[string]interface{}, argType string) map[string]string {
	if args, ok := args[argType]; ok {
		if labels, ok := args.(map[string]interface{}); ok {
			result := make(map[string]string, len(labels))
			for k, v := range labels {
				if v, ok := v.(string); ok {
					result[k] = v
				}
			}
			return result
		}
	}
	return nil
}

// createCNIConfigListFromNetwork will create a cni config file from the given network.
// It returns the cni config and the path to the file where the config was written.
// Set writeToDisk to false to only add this network into memory.
func (n *cniNetwork) createCNIConfigListFromNetwork(network *types.Network, writeToDisk bool) (*libcni.NetworkConfigList, string, error) {
	var (
		routes     []ipamRoute
		ipamRanges [][]ipamLocalHostRangeConf
		ipamConf   ipamConfig
		err        error
	)
	if len(network.Subnets) > 0 {
		for _, subnet := range network.Subnets {
			route, err := newIPAMDefaultRoute(util.IsIPv6(subnet.Subnet.IP))
			if err != nil {
				return nil, "", err
			}
			routes = append(routes, route)
			ipam := newIPAMLocalHostRange(subnet.Subnet, subnet.LeaseRange, subnet.Gateway)
			ipamRanges = append(ipamRanges, []ipamLocalHostRangeConf{*ipam})
		}
		ipamConf = newIPAMHostLocalConf(routes, ipamRanges)
	} else {
		ipamConf = ipamConfig{PluginType: "dhcp"}
	}

	vlan := 0
	mtu := 0
	for k, v := range network.Options {
		switch k {
		case "mtu":
			mtu, err = parseMTU(v)
			if err != nil {
				return nil, "", err
			}

		case "vlan":
			vlan, err = parseVlan(v)
			if err != nil {
				return nil, "", err
			}

		default:
			return nil, "", errors.Errorf("unsupported network option %s", k)
		}
	}

	isGateway := true
	ipMasq := true
	if network.Internal {
		isGateway = false
		ipMasq = false
	}
	// create CNI plugin configuration
	ncList := newNcList(network.Name, version.Current(), network.Labels, network.Options)
	var plugins []interface{}

	switch network.Driver {
	case types.BridgeNetworkDriver:
		bridge := newHostLocalBridge(network.NetworkInterface, isGateway, ipMasq, mtu, vlan, ipamConf)
		plugins = append(plugins, bridge, newPortMapPlugin(), newFirewallPlugin(), newTuningPlugin())
		// if we find the dnsname plugin we add configuration for it
		if hasDNSNamePlugin(n.cniPluginDirs) && network.DNSEnabled {
			// Note: in the future we might like to allow for dynamic domain names
			plugins = append(plugins, newDNSNamePlugin(defaultPodmanDomainName))
		}
		// Add the podman-machine CNI plugin if we are in a machine
		if n.isMachine {
			plugins = append(plugins, newPodmanMachinePlugin())
		}

	case types.MacVLANNetworkDriver:
		plugins = append(plugins, newMacVLANPlugin(network.NetworkInterface, mtu, ipamConf))

	default:
		return nil, "", errors.Errorf("driver %q is not supported by cni", network.Driver)
	}
	ncList["plugins"] = plugins
	b, err := json.MarshalIndent(ncList, "", "   ")
	if err != nil {
		return nil, "", err
	}
	cniPathName := ""
	if writeToDisk {
		cniPathName = filepath.Join(n.cniConfigDir, network.Name+".conflist")
		err = ioutil.WriteFile(cniPathName, b, 0644)
		if err != nil {
			return nil, "", err
		}
		f, err := os.Stat(cniPathName)
		if err != nil {
			return nil, "", err
		}
		stat := f.Sys().(*syscall.Stat_t)
		network.Created = time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
	} else {
		network.Created = time.Now()
	}
	config, err := libcni.ConfListFromBytes(b)
	if err != nil {
		return nil, "", err
	}
	return config, cniPathName, nil
}

// parseMTU parses the mtu option
func parseMTU(mtu string) (int, error) {
	if mtu == "" {
		return 0, nil // default
	}
	m, err := strconv.Atoi(mtu)
	if err != nil {
		return 0, err
	}
	if m < 0 {
		return 0, errors.Errorf("mtu %d is less than zero", m)
	}
	return m, nil
}

// parseVlan parses the vlan option
func parseVlan(vlan string) (int, error) {
	if vlan == "" {
		return 0, nil // default
	}
	v, err := strconv.Atoi(vlan)
	if err != nil {
		return 0, err
	}
	if v < 0 || v > 4094 {
		return 0, errors.Errorf("vlan ID %d must be between 0 and 4094", v)
	}
	return v, nil
}

func convertSpecgenPortsToCNIPorts(ports []types.PortMapping) ([]cniPortMapEntry, error) {
	cniPorts := make([]cniPortMapEntry, 0, len(ports))
	for _, port := range ports {
		if port.Protocol == "" {
			return nil, errors.New("port protocol should not be empty")
		}
		protocols := strings.Split(port.Protocol, ",")

		for _, protocol := range protocols {
			if !pkgutil.StringInSlice(protocol, []string{"tcp", "udp", "sctp"}) {
				return nil, errors.Errorf("unknown port protocol %s", protocol)
			}
			cniPort := cniPortMapEntry{
				HostPort:      int(port.HostPort),
				ContainerPort: int(port.ContainerPort),
				HostIP:        port.HostIP,
				Protocol:      protocol,
			}
			cniPorts = append(cniPorts, cniPort)
			for i := 1; i < int(port.Range); i++ {
				cniPort := cniPortMapEntry{
					HostPort:      int(port.HostPort) + i,
					ContainerPort: int(port.ContainerPort) + i,
					HostIP:        port.HostIP,
					Protocol:      protocol,
				}
				cniPorts = append(cniPorts, cniPort)
			}
		}
	}
	return cniPorts, nil
}
