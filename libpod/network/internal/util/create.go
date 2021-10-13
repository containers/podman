package util

import (
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/pkg/errors"
)

func CommonNetworkCreate(n NetUtil, network *types.Network) error {
	// FIXME: Should we use a different type for network create without the ID field?
	// the caller is not allowed to set a specific ID
	if network.ID != "" {
		return errors.Wrap(define.ErrInvalidArg, "ID can not be set for network create")
	}

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
		if !define.NameRegex.MatchString(network.Name) {
			return errors.Wrapf(define.RegexError, "network name %s invalid", network.Name)
		}
		if _, err := n.Network(network.Name); err == nil {
			return errors.Wrapf(define.ErrNetworkExists, "network name %s already used", network.Name)
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
