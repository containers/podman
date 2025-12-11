package etchosts

import (
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/libnetwork/util"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/machine"
	"github.com/containers/storage/pkg/unshare"
)

// GetHostContainersInternalIP return the host.containers.internal ip
// if netStatus is not nil then networkInterface also must be non nil otherwise this function panics
func GetHostContainersInternalIP(conf *config.Config, netStatus map[string]types.StatusBlock, networkInterface types.ContainerNetwork) string {
	switch conf.Containers.HostContainersInternalIP {
	case "":
		// if empty (default) we will automatically choose one below
		// if machine using gvproxy we let the gvproxy dns server handle the dns name so do not add it
		if machine.IsGvProxyBased() {
			return ""
		}
	case "none":
		return ""
	default:
		return conf.Containers.HostContainersInternalIP
	}
	ip := ""
	// Only use the bridge ip when root, as rootless the interfaces are created
	// inside the special netns and not the host so we cannot use them.
	if unshare.IsRootless() {
		return util.GetLocalIP()
	}
	for net, status := range netStatus {
		network, err := networkInterface.NetworkInspect(net)
		// only add the host entry for bridge networks
		// ip/macvlan gateway is normally not on the host
		if err != nil || network.Driver != types.BridgeNetworkDriver {
			continue
		}
		for _, netInt := range status.Interfaces {
			for _, netAddress := range netInt.Subnets {
				if netAddress.Gateway != nil {
					if util.IsIPv4(netAddress.Gateway) {
						return netAddress.Gateway.String()
					}
					// ipv6 address but keep looking since we prefer to use ipv4
					ip = netAddress.Gateway.String()
				}
			}
		}
	}
	if ip != "" {
		return ip
	}
	return util.GetLocalIP()
}

// GetNetworkHostEntries returns HostEntries for all ips in the network status
// with the given hostnames.
func GetNetworkHostEntries(netStatus map[string]types.StatusBlock, names ...string) HostEntries {
	hostEntries := make(HostEntries, 0, len(netStatus))
	for _, status := range netStatus {
		for _, netInt := range status.Interfaces {
			for _, netAddress := range netInt.Subnets {
				e := HostEntry{IP: netAddress.IPNet.IP.String(), Names: names}
				hostEntries = append(hostEntries, e)
			}
		}
	}
	return hostEntries
}
