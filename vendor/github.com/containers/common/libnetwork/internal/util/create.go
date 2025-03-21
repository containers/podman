package util

import (
	"fmt"

	"github.com/containers/common/libnetwork/types"
	"github.com/sirupsen/logrus"
)

func CommonNetworkCreate(n NetUtil, network *types.Network) error {
	if network.Labels == nil {
		network.Labels = map[string]string{}
	}
	if network.Options == nil {
		network.Options = map[string]string{}
	}
	if network.IPAMOptions == nil {
		network.IPAMOptions = map[string]string{}
	}

	var name string
	var err error
	// validate the name when given
	if network.Name != "" {
		if !types.NameRegex.MatchString(network.Name) {
			return fmt.Errorf("network name %s invalid: %w", network.Name, types.RegexError)
		}
		if _, err := n.Network(network.Name); err == nil {
			return fmt.Errorf("network name %s already used: %w", network.Name, types.ErrNetworkExists)
		}
	} else {
		name, err = GetFreeDeviceName(n)
		if err != nil {
			return err
		}
		network.Name = name
		// also use the name as interface name when we create a bridge network
		if network.Driver == types.BridgeNetworkDriver && network.NetworkInterface == "" {
			network.NetworkInterface = name
		}
	}
	return nil
}

func IpamNoneDisableDNS(network *types.Network) {
	if network.IPAMOptions[types.Driver] == types.NoneIPAMDriver {
		logrus.Debugf("dns disabled for network %q because ipam driver is set to none", network.Name)
		network.DNSEnabled = false
	}
}
