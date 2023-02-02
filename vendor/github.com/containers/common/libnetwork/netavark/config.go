//go:build linux || freebsd
// +build linux freebsd

package netavark

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	internalutil "github.com/containers/common/libnetwork/internal/util"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/util"
	"github.com/containers/storage/pkg/stringid"
)

// NetworkCreate will take a partial filled Network and fill the
// missing fields. It creates the Network and returns the full Network.
func (n *netavarkNetwork) NetworkCreate(net types.Network) (types.Network, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return types.Network{}, err
	}
	network, err := n.networkCreate(&net, false)
	if err != nil {
		return types.Network{}, err
	}
	// add the new network to the map
	n.networks[network.Name] = network
	return *network, nil
}

func (n *netavarkNetwork) networkCreate(newNetwork *types.Network, defaultNet bool) (*types.Network, error) {
	// if no driver is set use the default one
	if newNetwork.Driver == "" {
		newNetwork.Driver = types.DefaultNetworkDriver
	}
	if !defaultNet {
		// FIXME: Should we use a different type for network create without the ID field?
		// the caller is not allowed to set a specific ID
		if newNetwork.ID != "" {
			return nil, fmt.Errorf("ID can not be set for network create: %w", types.ErrInvalidArg)
		}

		// generate random network ID
		var i int
		for i = 0; i < 1000; i++ {
			id := stringid.GenerateNonCryptoID()
			if _, err := n.getNetwork(id); err != nil {
				newNetwork.ID = id
				break
			}
		}
		if i == 1000 {
			return nil, errors.New("failed to create random network ID")
		}
	}

	err := internalutil.CommonNetworkCreate(n, newNetwork)
	if err != nil {
		return nil, err
	}

	err = validateIPAMDriver(newNetwork)
	if err != nil {
		return nil, err
	}

	// Only get the used networks for validation if we do not create the default network.
	// The default network should not be validated against used subnets, we have to ensure
	// that this network can always be created even when a subnet is already used on the host.
	// This could happen if you run a container on this net, then the cni interface will be
	// created on the host and "block" this subnet from being used again.
	// Therefore the next podman command tries to create the default net again and it would
	// fail because it thinks the network is used on the host.
	var usedNetworks []*net.IPNet
	if !defaultNet && newNetwork.Driver == types.BridgeNetworkDriver {
		usedNetworks, err = internalutil.GetUsedSubnets(n)
		if err != nil {
			return nil, err
		}
	}

	switch newNetwork.Driver {
	case types.BridgeNetworkDriver:
		err = internalutil.CreateBridge(n, newNetwork, usedNetworks, n.defaultsubnetPools)
		if err != nil {
			return nil, err
		}
		// validate the given options, we do not need them but just check to make sure they are valid
		for key, value := range newNetwork.Options {
			switch key {
			case types.MTUOption:
				_, err = internalutil.ParseMTU(value)
				if err != nil {
					return nil, err
				}

			case types.VLANOption:
				_, err = internalutil.ParseVlan(value)
				if err != nil {
					return nil, err
				}

			case types.IsolateOption:
				val, err := strconv.ParseBool(value)
				if err != nil {
					return nil, err
				}
				// rust only support "true" or "false" while go can parse 1 and 0 as well so we need to change it
				newNetwork.Options[types.IsolateOption] = strconv.FormatBool(val)
			default:
				return nil, fmt.Errorf("unsupported bridge network option %s", key)
			}
		}
	case types.MacVLANNetworkDriver:
		err = createMacvlan(newNetwork)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported driver %s: %w", newNetwork.Driver, types.ErrInvalidArg)
	}

	// when we do not have ipam we must disable dns
	internalutil.IpamNoneDisableDNS(newNetwork)

	// add gateway when not internal or dns enabled
	addGateway := !newNetwork.Internal || newNetwork.DNSEnabled
	err = internalutil.ValidateSubnets(newNetwork, addGateway, usedNetworks)
	if err != nil {
		return nil, err
	}

	newNetwork.Created = time.Now()

	if !defaultNet {
		confPath := filepath.Join(n.networkConfigDir, newNetwork.Name+".json")
		f, err := os.Create(confPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "     ")
		err = enc.Encode(newNetwork)
		if err != nil {
			return nil, err
		}
	}

	return newNetwork, nil
}

func createMacvlan(network *types.Network) error {
	if network.NetworkInterface != "" {
		interfaceNames, err := internalutil.GetLiveNetworkNames()
		if err != nil {
			return err
		}
		if !util.StringInSlice(network.NetworkInterface, interfaceNames) {
			return fmt.Errorf("parent interface %s does not exist", network.NetworkInterface)
		}
	}

	// we already validated the drivers before so we just have to set the default here
	switch network.IPAMOptions[types.Driver] {
	case "":
		if len(network.Subnets) == 0 {
			return fmt.Errorf("macvlan driver needs at least one subnet specified, DHCP is not yet supported with netavark")
		}
		network.IPAMOptions[types.Driver] = types.HostLocalIPAMDriver
	case types.HostLocalIPAMDriver:
		if len(network.Subnets) == 0 {
			return fmt.Errorf("macvlan driver needs at least one subnet specified, when the host-local ipam driver is set")
		}
	}

	// validate the given options, we do not need them but just check to make sure they are valid
	for key, value := range network.Options {
		switch key {
		case types.ModeOption:
			if !util.StringInSlice(value, types.ValidMacVLANModes) {
				return fmt.Errorf("unknown macvlan mode %q", value)
			}
		case types.MTUOption:
			_, err := internalutil.ParseMTU(value)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported macvlan network option %s", key)
		}
	}
	return nil
}

// NetworkRemove will remove the Network with the given name or ID.
// It does not ensure that the network is unused.
func (n *netavarkNetwork) NetworkRemove(nameOrID string) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return err
	}

	network, err := n.getNetwork(nameOrID)
	if err != nil {
		return err
	}

	// Removing the default network is not allowed.
	if network.Name == n.defaultNetwork {
		return fmt.Errorf("default network %s cannot be removed", n.defaultNetwork)
	}

	file := filepath.Join(n.networkConfigDir, network.Name+".json")
	// make sure to not error for ErrNotExist
	if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(n.networks, network.Name)
	return nil
}

// NetworkList will return all known Networks. Optionally you can
// supply a list of filter functions. Only if a network matches all
// functions it is returned.
func (n *netavarkNetwork) NetworkList(filters ...types.FilterFunc) ([]types.Network, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return nil, err
	}

	networks := make([]types.Network, 0, len(n.networks))
outer:
	for _, net := range n.networks {
		for _, filter := range filters {
			// All filters have to match, if one does not match we can skip to the next network.
			if !filter(*net) {
				continue outer
			}
		}
		networks = append(networks, *net)
	}
	return networks, nil
}

// NetworkInspect will return the Network with the given name or ID.
func (n *netavarkNetwork) NetworkInspect(nameOrID string) (types.Network, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return types.Network{}, err
	}

	network, err := n.getNetwork(nameOrID)
	if err != nil {
		return types.Network{}, err
	}
	return *network, nil
}

func validateIPAMDriver(n *types.Network) error {
	ipamDriver := n.IPAMOptions[types.Driver]
	switch ipamDriver {
	case "", types.HostLocalIPAMDriver:
	case types.NoneIPAMDriver:
		if len(n.Subnets) > 0 {
			return errors.New("none ipam driver is set but subnets are given")
		}
	case types.DHCPIPAMDriver:
		return errors.New("dhcp ipam driver is not yet supported with netavark")
	default:
		return fmt.Errorf("unsupported ipam driver %q", ipamDriver)
	}
	return nil
}
