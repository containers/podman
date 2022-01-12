// +build linux

package cni

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/common/libnetwork/types"
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

	// modTime is the timestamp when the config dir was modified
	modTime time.Time

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
}

// NewCNINetworkInterface creates the ContainerNetwork interface for the CNI backend.
// Note: The networks are not loaded from disk until a method is called.
func NewCNINetworkInterface(conf *InitConfig) (types.ContainerNetwork, error) {
	// TODO: consider using a shared memory lock
	lock, err := lockfile.GetLockfile(filepath.Join(conf.CNIConfigDir, "cni.lock"))
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

// Drivers will return the list of supported network drivers
// for this interface.
func (n *cniNetwork) Drivers() []string {
	return []string{types.BridgeNetworkDriver, types.MacVLANNetworkDriver, types.IPVLANNetworkDriver}
}

// DefaultNetworkName will return the default cni network name.
func (n *cniNetwork) DefaultNetworkName() string {
	return n.defaultNetwork
}

func (n *cniNetwork) loadNetworks() error {
	// check the mod time of the config dir
	f, err := os.Stat(n.cniConfigDir)
	if err != nil {
		return err
	}
	modTime := f.ModTime()

	// skip loading networks if they are already loaded and
	// if the config dir was not modified since the last call
	if n.networks != nil && modTime.Equal(n.modTime) {
		return nil
	}
	// make sure the remove all networks before we reload them
	n.networks = nil
	n.modTime = modTime

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
			if !errors.Is(err, os.ErrNotExist) {
				logrus.Warnf("Error loading CNI config file %s: %v", file, err)
			}
			continue
		}

		if !types.NameRegex.MatchString(conf.Name) {
			logrus.Warnf("CNI config list %s has invalid name, skipping: %v", file, types.RegexError)
			continue
		}

		// podman < v4.0 used the podman-machine cni plugin for podman machine port forwarding
		// since this is now build into podman we no longer use the plugin
		// old configs may still contain it so we just remove it here
		if n.isMachine {
			conf = removeMachinePlugin(conf)
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
		logrus.Debugf("Successfully loaded network %s: %v", net.Name, net)
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
	return n.networkCreate(&net, true)
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
	return nil, errors.Wrapf(types.ErrNoSuchNetwork, "unable to find network with name or ID %s", nameOrID)
}

// getNetworkIDFromName creates a network ID from the name. It is just the
// sha256 hash so it is not safe but it should be safe enough for our use case.
func getNetworkIDFromName(name string) string {
	hash := sha256.Sum256([]byte(name))
	return hex.EncodeToString(hash[:])
}

// Implement the NetUtil interface for easy code sharing with other network interfaces.

// ForEach call the given function for each network
func (n *cniNetwork) ForEach(run func(types.Network)) {
	for _, val := range n.networks {
		run(*val.libpodNet)
	}
}

// Len return the number of networks
func (n *cniNetwork) Len() int {
	return len(n.networks)
}

// DefaultInterfaceName return the default cni bridge name, must be suffixed with a number.
func (n *cniNetwork) DefaultInterfaceName() string {
	return cniDeviceName
}

func (n *cniNetwork) Network(nameOrID string) (*types.Network, error) {
	network, err := n.getNetwork(nameOrID)
	if err != nil {
		return nil, err
	}
	return network.libpodNet, err
}
