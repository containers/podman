// +build linux

package cni

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/libpod/network/util"
	pkgutil "github.com/containers/podman/v3/pkg/util"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type cniNetwork struct {
	// cniConfigDir is directory where the cni config files are stored.
	cniConfigDir string
	// cniPluginDirs is a list of directories where cni should look for the plugins.
	cniPluginDirs []string

	cniConf *libcni.CNIConfig

	// defaultNetwork is the name for the default network.
	defaultNetwork string
	// defaultSubnet is the default subnet for the default network.
	defaultSubnet types.IPNet

	// isMachine describes whenever podman runs in a podman machine environment.
	isMachine bool

	// lock is a internal lock for critical operations
	lock lockfile.Locker

	// networks is a map with loaded networks, the key is the network name
	networks map[string]*network
}

type network struct {
	// filename is the full path to the cni config file on disk
	filename  string
	libpodNet *types.Network
	cniNet    *libcni.NetworkConfigList
}

type InitConfig struct {
	// CNIConfigDir is directory where the cni config files are stored.
	CNIConfigDir string
	// CNIPluginDirs is a list of directories where cni should look for the plugins.
	CNIPluginDirs []string

	// DefaultNetwork is the name for the default network.
	DefaultNetwork string
	// DefaultSubnet is the default subnet for the default network.
	DefaultSubnet string

	// IsMachine describes whenever podman runs in a podman machine environment.
	IsMachine bool

	// LockFile is the path to lock file.
	LockFile string
}

// NewCNINetworkInterface creates the ContainerNetwork interface for the CNI backend.
// Note: The networks are not loaded from disk until a method is called.
func NewCNINetworkInterface(conf InitConfig) (types.ContainerNetwork, error) {
	// TODO: consider using a shared memory lock
	lock, err := lockfile.GetLockfile(conf.LockFile)
	if err != nil {
		return nil, err
	}

	defaultNetworkName := conf.DefaultNetwork
	if defaultNetworkName == "" {
		defaultNetworkName = types.DefaultNetworkName
	}

	defaultSubnet := conf.DefaultSubnet
	if defaultSubnet == "" {
		defaultSubnet = types.DefaultSubnet
	}
	defaultNet, err := types.ParseCIDR(defaultSubnet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse default subnet")
	}

	cni := libcni.NewCNIConfig(conf.CNIPluginDirs, &cniExec{})
	n := &cniNetwork{
		cniConfigDir:   conf.CNIConfigDir,
		cniPluginDirs:  conf.CNIPluginDirs,
		cniConf:        cni,
		defaultNetwork: defaultNetworkName,
		defaultSubnet:  defaultNet,
		isMachine:      conf.IsMachine,
		lock:           lock,
	}

	return n, nil
}

func (n *cniNetwork) loadNetworks() error {
	// skip loading networks if they are already loaded
	if n.networks != nil {
		return nil
	}
	// FIXME: do we have to support other file types as well, e.g. .conf?
	files, err := libcni.ConfFiles(n.cniConfigDir, []string{".conflist"})
	if err != nil {
		return err
	}
	networks := make(map[string]*network, len(files))
	for _, file := range files {
		conf, err := libcni.ConfListFromFile(file)
		if err != nil {
			// do not log ENOENT errors
			if !os.IsNotExist(err) {
				logrus.Warnf("Error loading CNI config file %s: %v", file, err)
			}
			continue
		}

		if !define.NameRegex.MatchString(conf.Name) {
			logrus.Warnf("CNI config list %s has invalid name, skipping: %v", file, define.RegexError)
			continue
		}

		if _, err := n.cniConf.ValidateNetworkList(context.Background(), conf); err != nil {
			logrus.Warnf("Error validating CNI config file %s: %v", file, err)
			continue
		}

		if val, ok := networks[conf.Name]; ok {
			logrus.Warnf("CNI config list %s has the same network name as %s, skipping", file, val.filename)
			continue
		}

		net, err := createNetworkFromCNIConfigList(conf, file)
		if err != nil {
			logrus.Errorf("CNI config list %s could not be converted to a libpod config, skipping: %v", file, err)
			continue
		}
		logrus.Tracef("Successfully loaded network %s: %v", net.Name, net)
		networkInfo := network{
			filename:  file,
			cniNet:    conf,
			libpodNet: net,
		}
		networks[net.Name] = &networkInfo
	}

	// create the default network in memory if it did not exists on disk
	if networks[n.defaultNetwork] == nil {
		networkInfo, err := n.createDefaultNetwork()
		if err != nil {
			return errors.Wrapf(err, "failed to create default network %s", n.defaultNetwork)
		}
		networks[n.defaultNetwork] = networkInfo
	}

	logrus.Debugf("Successfully loaded %d networks", len(networks))
	n.networks = networks
	return nil
}

func (n *cniNetwork) createDefaultNetwork() (*network, error) {
	net := types.Network{
		Name:             n.defaultNetwork,
		NetworkInterface: "cni-podman0",
		Driver:           types.BridgeNetworkDriver,
		Subnets: []types.Subnet{
			{Subnet: n.defaultSubnet},
		},
	}
	return n.networkCreate(net, false)
}

// getNetwork will lookup a network by name or ID. It returns an
// error when no network was found or when more than one network
// with the given (partial) ID exists.
// getNetwork will read from the networks map, therefore the caller
// must ensure that n.lock is locked before using it.
func (n *cniNetwork) getNetwork(nameOrID string) (*network, error) {
	// fast path check the map key, this will only work for names
	if val, ok := n.networks[nameOrID]; ok {
		return val, nil
	}
	// If there was no match we might got a full or partial ID.
	var net *network
	for _, val := range n.networks {
		// This should not happen because we already looked up the map by name but check anyway.
		if val.libpodNet.Name == nameOrID {
			return val, nil
		}

		if strings.HasPrefix(val.libpodNet.ID, nameOrID) {
			if net != nil {
				return nil, errors.Errorf("more than one result for network ID %s", nameOrID)
			}
			net = val
		}
	}
	if net != nil {
		return net, nil
	}
	return nil, errors.Wrapf(define.ErrNoSuchNetwork, "unable to find network with name or ID %s", nameOrID)
}

// getNetworkIDFromName creates a network ID from the name. It is just the
// sha256 hash so it is not safe but it should be safe enough for our use case.
func getNetworkIDFromName(name string) string {
	hash := sha256.Sum256([]byte(name))
	return hex.EncodeToString(hash[:])
}

// getFreeIPv6NetworkSubnet returns a unused ipv4 subnet
func (n *cniNetwork) getFreeIPv4NetworkSubnet() (*types.Subnet, error) {
	networks, err := n.getUsedSubnets()
	if err != nil {
		return nil, err
	}

	// the default podman network is 10.88.0.0/16
	// start locking for free /24 networks
	network := &net.IPNet{
		IP:   net.IP{10, 89, 0, 0},
		Mask: net.IPMask{255, 255, 255, 0},
	}

	// TODO: make sure to not use public subnets
	for {
		if intersectsConfig := util.NetworkIntersectsWithNetworks(network, networks); !intersectsConfig {
			logrus.Debugf("found free ipv4 network subnet %s", network.String())
			return &types.Subnet{
				Subnet: types.IPNet{IPNet: *network},
			}, nil
		}
		network, err = util.NextSubnet(network)
		if err != nil {
			return nil, err
		}
	}
}

// getFreeIPv6NetworkSubnet returns a unused ipv6 subnet
func (n *cniNetwork) getFreeIPv6NetworkSubnet() (*types.Subnet, error) {
	networks, err := n.getUsedSubnets()
	if err != nil {
		return nil, err
	}

	// FIXME: Is 10000 fine as limit? We should prevent an endless loop.
	for i := 0; i < 10000; i++ {
		// RFC4193: Choose the ipv6 subnet random and NOT sequentially.
		network, err := util.GetRandomIPv6Subnet()
		if err != nil {
			return nil, err
		}
		if intersectsConfig := util.NetworkIntersectsWithNetworks(&network, networks); !intersectsConfig {
			logrus.Debugf("found free ipv6 network subnet %s", network.String())
			return &types.Subnet{
				Subnet: types.IPNet{IPNet: network},
			}, nil
		}
	}
	return nil, errors.New("failed to get random ipv6 subnet")
}

// getUsedSubnets returns a list of all used subnets by network
// configs and interfaces on the host.
func (n *cniNetwork) getUsedSubnets() ([]*net.IPNet, error) {
	// first, load all used subnets from network configs
	subnets := make([]*net.IPNet, 0, len(n.networks))
	for _, val := range n.networks {
		for _, subnet := range val.libpodNet.Subnets {
			// nolint:exportloopref
			subnets = append(subnets, &subnet.Subnet.IPNet)
		}
	}
	// second, load networks from the current system
	liveSubnets, err := util.GetLiveNetworkSubnets()
	if err != nil {
		return nil, err
	}
	return append(subnets, liveSubnets...), nil
}

// getFreeDeviceName returns a free device name which can
// be used for new configs as name and bridge interface name
func (n *cniNetwork) getFreeDeviceName() (string, error) {
	bridgeNames := n.getBridgeInterfaceNames()
	netNames := n.getUsedNetworkNames()
	liveInterfaces, err := util.GetLiveNetworkNames()
	if err != nil {
		return "", nil
	}
	names := make([]string, 0, len(bridgeNames)+len(netNames)+len(liveInterfaces))
	names = append(names, bridgeNames...)
	names = append(names, netNames...)
	names = append(names, liveInterfaces...)
	// FIXME: Is a limit fine?
	// Start by 1, 0 is reserved for the default network
	for i := 1; i < 1000000; i++ {
		deviceName := fmt.Sprintf("%s%d", cniDeviceName, i)
		if !pkgutil.StringInSlice(deviceName, names) {
			logrus.Debugf("found free device name %s", deviceName)
			return deviceName, nil
		}
	}
	return "", errors.New("could not find free device name, to many iterations")
}

// getUsedNetworkNames returns all network names already used
// by network configs
func (n *cniNetwork) getUsedNetworkNames() []string {
	names := make([]string, 0, len(n.networks))
	for _, val := range n.networks {
		names = append(names, val.libpodNet.Name)
	}
	return names
}

// getUsedNetworkNames returns all bridge device names already used
// by network configs
func (n *cniNetwork) getBridgeInterfaceNames() []string {
	names := make([]string, 0, len(n.networks))
	for _, val := range n.networks {
		if val.libpodNet.Driver == types.BridgeNetworkDriver {
			names = append(names, val.libpodNet.NetworkInterface)
		}
	}
	return names
}
