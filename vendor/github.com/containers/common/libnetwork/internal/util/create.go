package util

import (
	"github.com/containers/common/libnetwork/types"
	"github.com/pkg/errors"
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
			return errors.Wrapf(types.RegexError, "network name %s invalid", network.Name)
		}
		if _, err := n.Network(network.Name); err == nil {
			return errors.Wrapf(types.ErrNetworkExists, "network name %s already used", network.Name)
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
