package util

import (
	"net"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/libnetwork/util"
	pkgutil "github.com/containers/common/pkg/util"
	"github.com/pkg/errors"
)

func CreateBridge(n NetUtil, network *types.Network, usedNetworks []*net.IPNet) error {
	if network.NetworkInterface != "" {
		bridges := GetBridgeInterfaceNames(n)
		if pkgutil.StringInSlice(network.NetworkInterface, bridges) {
			return errors.Errorf("bridge name %s already in use", network.NetworkInterface)
		}
		if !types.NameRegex.MatchString(network.NetworkInterface) {
			return errors.Wrapf(types.RegexError, "bridge name %s invalid", network.NetworkInterface)
		}
	} else {
		var err error
		network.NetworkInterface, err = GetFreeDeviceName(n)
		if err != nil {
			return err
		}
	}

	if network.IPAMOptions["driver"] != types.DHCPIPAMDriver {
		if len(network.Subnets) == 0 {
			freeSubnet, err := GetFreeIPv4NetworkSubnet(usedNetworks)
			if err != nil {
				return err
			}
			network.Subnets = append(network.Subnets, *freeSubnet)
		}
		// ipv6 enabled means dual stack, check if we already have
		// a ipv4 or ipv6 subnet and add one if not.
		if network.IPv6Enabled {
			ipv4 := false
			ipv6 := false
			for _, subnet := range network.Subnets {
				if util.IsIPv6(subnet.Subnet.IP) {
					ipv6 = true
				}
				if util.IsIPv4(subnet.Subnet.IP) {
					ipv4 = true
				}
			}
			if !ipv4 {
				freeSubnet, err := GetFreeIPv4NetworkSubnet(usedNetworks)
				if err != nil {
					return err
				}
				network.Subnets = append(network.Subnets, *freeSubnet)
			}
			if !ipv6 {
				freeSubnet, err := GetFreeIPv6NetworkSubnet(usedNetworks)
				if err != nil {
					return err
				}
				network.Subnets = append(network.Subnets, *freeSubnet)
			}
		}
		network.IPAMOptions["driver"] = types.HostLocalIPAMDriver
	}
	return nil
}
