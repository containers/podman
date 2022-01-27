// +build linux

package netavark

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/common/libnetwork/internal/util"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/containers/storage/pkg/unshare"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type netavarkNetwork struct {
	// networkConfigDir is directory where the network config files are stored.
	networkConfigDir string

	// networkRunDir is where temporary files are stored, i.e.the ipam db, aardvark config etc
	networkRunDir string

	// tells netavark whether this is rootless mode or rootfull, "true" or "false"
	networkRootless bool

	// netavarkBinary is the path to the netavark binary.
	netavarkBinary string
	// aardvarkBinary is the path to the aardvark binary.
	aardvarkBinary string

	// defaultNetwork is the name for the default network.
	defaultNetwork string
	// defaultSubnet is the default subnet for the default network.
	defaultSubnet types.IPNet

	// ipamDBPath is the path to the ip allocation bolt db
	ipamDBPath string

	// syslog describes whenever the netavark debbug output should be log to the syslog as well.
	// This will use logrus to do so, make sure logrus is set up to log to the syslog.
	syslog bool

	// lock is a internal lock for critical operations
	lock lockfile.Locker

	// modTime is the timestamp when the config dir was modified
	modTime time.Time

	// networks is a map with loaded networks, the key is the network name
	networks map[string]*types.Network
}

type InitConfig struct {
	// NetworkConfigDir is directory where the network config files are stored.
	NetworkConfigDir string

	// NetavarkBinary is the path to the netavark binary.
	NetavarkBinary string
	// AardvarkBinary is the path to the aardvark binary.
	AardvarkBinary string

	// NetworkRunDir is where temporary files are stored, i.e.the ipam db, aardvark config
	NetworkRunDir string

	// DefaultNetwork is the name for the default network.
	DefaultNetwork string
	// DefaultSubnet is the default subnet for the default network.
	DefaultSubnet string

	// Syslog describes whenever the netavark debbug output should be log to the syslog as well.
	// This will use logrus to do so, make sure logrus is set up to log to the syslog.
	Syslog bool
}

// NewNetworkInterface creates the ContainerNetwork interface for the netavark backend.
// Note: The networks are not loaded from disk until a method is called.
func NewNetworkInterface(conf *InitConfig) (types.ContainerNetwork, error) {
	// TODO: consider using a shared memory lock
	lock, err := lockfile.GetLockfile(filepath.Join(conf.NetworkConfigDir, "netavark.lock"))
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

	if err := os.MkdirAll(conf.NetworkConfigDir, 0755); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(conf.NetworkRunDir, 0755); err != nil {
		return nil, err
	}

	n := &netavarkNetwork{
		networkConfigDir: conf.NetworkConfigDir,
		networkRunDir:    conf.NetworkRunDir,
		netavarkBinary:   conf.NetavarkBinary,
		aardvarkBinary:   conf.AardvarkBinary,
		networkRootless:  unshare.IsRootless(),
		ipamDBPath:       filepath.Join(conf.NetworkRunDir, "ipam.db"),
		defaultNetwork:   defaultNetworkName,
		defaultSubnet:    defaultNet,
		lock:             lock,
		syslog:           conf.Syslog,
	}

	return n, nil
}

// Drivers will return the list of supported network drivers
// for this interface.
func (n *netavarkNetwork) Drivers() []string {
	return []string{types.BridgeNetworkDriver, types.MacVLANNetworkDriver}
}

// DefaultNetworkName will return the default netavark network name.
func (n *netavarkNetwork) DefaultNetworkName() string {
	return n.defaultNetwork
}

func (n *netavarkNetwork) loadNetworks() error {
	// check the mod time of the config dir
	f, err := os.Stat(n.networkConfigDir)
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

	files, err := ioutil.ReadDir(n.networkConfigDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	networks := make(map[string]*types.Network, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}

		path := filepath.Join(n.networkConfigDir, f.Name())
		file, err := os.Open(path)
		if err != nil {
			// do not log ENOENT errors
			if !errors.Is(err, os.ErrNotExist) {
				logrus.Warnf("Error loading network config file %q: %v", path, err)
			}
			continue
		}
		network := new(types.Network)
		err = json.NewDecoder(file).Decode(network)
		if err != nil {
			logrus.Warnf("Error reading network config file %q: %v", path, err)
			continue
		}

		// check that the filename matches the network name
		if network.Name+".json" != f.Name() {
			logrus.Warnf("Network config name %q does not match file name %q, skipping", network.Name, f.Name())
			continue
		}

		if !types.NameRegex.MatchString(network.Name) {
			logrus.Warnf("Network config %q has invalid name: %q, skipping: %v", path, network.Name, types.RegexError)
			continue
		}

		err = parseNetwork(network)
		if err != nil {
			logrus.Warnf("Network config %q could not be parsed, skipping: %v", path, err)
			continue
		}

		logrus.Debugf("Successfully loaded network %s: %v", network.Name, network)
		networks[network.Name] = network
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

func parseNetwork(network *types.Network) error {
	if network.Labels == nil {
		network.Labels = map[string]string{}
	}
	if network.Options == nil {
		network.Options = map[string]string{}
	}
	if network.IPAMOptions == nil {
		network.IPAMOptions = map[string]string{}
	}

	if len(network.ID) != 64 {
		return errors.Errorf("invalid network ID %q", network.ID)
	}

	return util.ValidateSubnets(network, nil)
}

func (n *netavarkNetwork) createDefaultNetwork() (*types.Network, error) {
	net := types.Network{
		Name:             n.defaultNetwork,
		NetworkInterface: defaultBridgeName + "0",
		// Important do not change this ID
		ID:     "2f259bab93aaaaa2542ba43ef33eb990d0999ee1b9924b557b7be53c0b7a1bb9",
		Driver: types.BridgeNetworkDriver,
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
func (n *netavarkNetwork) getNetwork(nameOrID string) (*types.Network, error) {
	// fast path check the map key, this will only work for names
	if val, ok := n.networks[nameOrID]; ok {
		return val, nil
	}
	// If there was no match we might got a full or partial ID.
	var net *types.Network
	for _, val := range n.networks {
		// This should not happen because we already looked up the map by name but check anyway.
		if val.Name == nameOrID {
			return val, nil
		}

		if strings.HasPrefix(val.ID, nameOrID) {
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

// Implement the NetUtil interface for easy code sharing with other network interfaces.

// ForEach call the given function for each network
func (n *netavarkNetwork) ForEach(run func(types.Network)) {
	for _, val := range n.networks {
		run(*val)
	}
}

// Len return the number of networks
func (n *netavarkNetwork) Len() int {
	return len(n.networks)
}

// DefaultInterfaceName return the default cni bridge name, must be suffixed with a number.
func (n *netavarkNetwork) DefaultInterfaceName() string {
	return defaultBridgeName
}

func (n *netavarkNetwork) Network(nameOrID string) (*types.Network, error) {
	network, err := n.getNetwork(nameOrID)
	if err != nil {
		return nil, err
	}
	return network, nil
}
