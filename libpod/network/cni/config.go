// +build linux

package cni

import (
	"net"
	"os"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/libpod/network/util"
	pkgutil "github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// NetworkCreate will take a partial filled Network and fill the
// missing fields. It creates the Network and returns the full Network.
func (n *cniNetwork) NetworkCreate(net types.Network) (types.Network, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return types.Network{}, err
	}
	network, err := n.networkCreate(net, true)
	if err != nil {
		return types.Network{}, err
	}
	// add the new network to the map
	n.networks[network.libpodNet.Name] = network
	return *network.libpodNet, nil
}

func (n *cniNetwork) networkCreate(net types.Network, writeToDisk bool) (*network, error) {
	// if no driver is set use the default one
	if net.Driver == "" {
		net.Driver = types.DefaultNetworkDriver
	}

	// FIXME: Should we use a different type for network create without the ID field?
	// the caller is not allowed to set a specific ID
	if net.ID != "" {
		return nil, errors.Wrap(define.ErrInvalidArg, "ID can not be set for network create")
	}

	if net.Labels == nil {
		net.Labels = map[string]string{}
	}
	if net.Options == nil {
		net.Options = map[string]string{}
	}
	if net.IPAMOptions == nil {
		net.IPAMOptions = map[string]string{}
	}

	var name string
	var err error
	// validate the name when given
	if net.Name != "" {
		if !define.NameRegex.MatchString(net.Name) {
			return nil, errors.Wrapf(define.RegexError, "network name %s invalid", net.Name)
		}
		if _, ok := n.networks[net.Name]; ok {
			return nil, errors.Wrapf(define.ErrNetworkExists, "network name %s already used", net.Name)
		}
	} else {
		name, err = n.getFreeDeviceName()
		if err != nil {
			return nil, err
		}
		net.Name = name
	}

	switch net.Driver {
	case types.BridgeNetworkDriver:
		// if the name was created with getFreeDeviceName set the interface to it as well
		if name != "" && net.NetworkInterface == "" {
			net.NetworkInterface = name
		}
		err = n.createBridge(&net)
		if err != nil {
			return nil, err
		}
	case types.MacVLANNetworkDriver:
		err = createMacVLAN(&net)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.Wrapf(define.ErrInvalidArg, "unsupported driver %s", net.Driver)
	}

	for i := range net.Subnets {
		err := validateSubnet(&net.Subnets[i], !net.Internal)
		if err != nil {
			return nil, err
		}
		if util.IsIPv6(net.Subnets[i].Subnet.IP) {
			net.IPv6Enabled = true
		}
	}

	// generate the network ID
	net.ID = getNetworkIDFromName(net.Name)

	// FIXME: Should this be a hard error?
	if net.DNSEnabled && net.Internal && hasDNSNamePlugin(n.cniPluginDirs) {
		logrus.Warnf("dnsname and internal networks are incompatible. dnsname plugin not configured for network %s", net.Name)
		net.DNSEnabled = false
	}

	cniConf, path, err := n.createCNIConfigListFromNetwork(&net, writeToDisk)
	if err != nil {
		return nil, err
	}
	return &network{cniNet: cniConf, libpodNet: &net, filename: path}, nil
}

// NetworkRemove will remove the Network with the given name or ID.
// It does not ensure that the network is unused.
func (n *cniNetwork) NetworkRemove(nameOrID string) error {
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
	if network.libpodNet.Name == n.defaultNetwork {
		return errors.Errorf("default network %s cannot be removed", n.defaultNetwork)
	}

	// Remove the bridge network interface on the host.
	if network.libpodNet.Driver == types.BridgeNetworkDriver {
		link, err := netlink.LinkByName(network.libpodNet.NetworkInterface)
		if err == nil {
			err = netlink.LinkDel(link)
			// only log the error, it is not fatal
			if err != nil {
				logrus.Infof("failed to remove network interface %s: %v", network.libpodNet.NetworkInterface, err)
			}
		}
	}

	file := network.filename
	delete(n.networks, network.libpodNet.Name)

	return os.Remove(file)
}

// NetworkList will return all known Networks. Optionally you can
// supply a list of filter functions. Only if a network matches all
// functions it is returned.
func (n *cniNetwork) NetworkList(filters ...types.FilterFunc) ([]types.Network, error) {
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
			if !filter(*net.libpodNet) {
				continue outer
			}
		}
		networks = append(networks, *net.libpodNet)
	}
	return networks, nil
}

// NetworkInspect will return the Network with the given name or ID.
func (n *cniNetwork) NetworkInspect(nameOrID string) (types.Network, error) {
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
	return *network.libpodNet, nil
}

func createMacVLAN(network *types.Network) error {
	if network.Internal {
		return errors.New("internal is not supported with macvlan")
	}
	if network.NetworkInterface != "" {
		interfaceNames, err := util.GetLiveNetworkNames()
		if err != nil {
			return err
		}
		if !pkgutil.StringInSlice(network.NetworkInterface, interfaceNames) {
			return errors.Errorf("parent interface %s does not exists", network.NetworkInterface)
		}
	}
	if len(network.Subnets) == 0 {
		network.IPAMOptions["driver"] = types.DHCPIPAMDriver
	} else {
		network.IPAMOptions["driver"] = types.HostLocalIPAMDriver
	}
	return nil
}

func (n *cniNetwork) createBridge(network *types.Network) error {
	if network.NetworkInterface != "" {
		bridges := n.getBridgeInterfaceNames()
		if pkgutil.StringInSlice(network.NetworkInterface, bridges) {
			return errors.Errorf("bridge name %s already in use", network.NetworkInterface)
		}
		if !define.NameRegex.MatchString(network.NetworkInterface) {
			return errors.Wrapf(define.RegexError, "bridge name %s invalid", network.NetworkInterface)
		}
	} else {
		var err error
		network.NetworkInterface, err = n.getFreeDeviceName()
		if err != nil {
			return err
		}
	}

	if len(network.Subnets) == 0 {
		freeSubnet, err := n.getFreeIPv4NetworkSubnet()
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
			freeSubnet, err := n.getFreeIPv4NetworkSubnet()
			if err != nil {
				return err
			}
			network.Subnets = append(network.Subnets, *freeSubnet)
		}
		if !ipv6 {
			freeSubnet, err := n.getFreeIPv6NetworkSubnet()
			if err != nil {
				return err
			}
			network.Subnets = append(network.Subnets, *freeSubnet)
		}
	}
	network.IPAMOptions["driver"] = types.HostLocalIPAMDriver
	return nil
}

// validateSubnet will validate a given Subnet. It checks if the
// given gateway and lease range are part of this subnet. If the
// gateway is empty and addGateway is true it will get the first
// available ip in the subnet assigned.
func validateSubnet(s *types.Subnet, addGateway bool) error {
	if s == nil {
		return errors.New("subnet is nil")
	}
	// Reparse to ensure subnet is valid.
	// Do not use types.ParseCIDR() because we want the ip to be
	// the network address and not a random ip in the subnet.
	_, net, err := net.ParseCIDR(s.Subnet.String())
	if err != nil {
		return errors.Wrap(err, "subnet invalid")
	}
	s.Subnet = types.IPNet{IPNet: *net}
	if s.Gateway != nil {
		if !s.Subnet.Contains(s.Gateway) {
			return errors.Errorf("gateway %s not in subnet %s", s.Gateway, &s.Subnet)
		}
	} else if addGateway {
		ip, err := util.FirstIPInSubnet(net)
		if err != nil {
			return err
		}
		s.Gateway = ip
	}
	if s.LeaseRange != nil {
		if s.LeaseRange.StartIP != nil && !s.Subnet.Contains(s.LeaseRange.StartIP) {
			return errors.Errorf("lease range start ip %s not in subnet %s", s.LeaseRange.StartIP, &s.Subnet)
		}
		if s.LeaseRange.EndIP != nil && !s.Subnet.Contains(s.LeaseRange.EndIP) {
			return errors.Errorf("lease range end ip %s not in subnet %s", s.LeaseRange.EndIP, &s.Subnet)
		}
	}
	return nil
}
