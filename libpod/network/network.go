package network

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// DefaultNetworkDriver is the default network type used
var DefaultNetworkDriver = "bridge"

// SupportedNetworkDrivers describes the list of supported drivers
var SupportedNetworkDrivers = []string{DefaultNetworkDriver}

// isSupportedDriver checks if the user provided driver is supported
func isSupportedDriver(driver string) error {
	if util.StringInSlice(driver, SupportedNetworkDrivers) {
		return nil
	}
	return errors.Errorf("driver '%s' is not supported", driver)
}

// GetLiveNetworks returns a slice of networks representing what the system
// has defined as network interfaces
func GetLiveNetworks() ([]*net.IPNet, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	nets := make([]*net.IPNet, 0, len(addrs))
	for _, address := range addrs {
		_, n, err := net.ParseCIDR(address.String())
		if err != nil {
			return nil, err
		}
		nets = append(nets, n)
	}
	return nets, nil
}

// GetLiveNetworkNames returns a list of network interfaces on the system
func GetLiveNetworkNames() ([]string, error) {
	liveInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	interfaceNames := make([]string, 0, len(liveInterfaces))
	for _, i := range liveInterfaces {
		interfaceNames = append(interfaceNames, i.Name)
	}
	return interfaceNames, nil
}

// GetFreeNetwork looks for a free network according to existing cni configuration
// files and network interfaces.
func GetFreeNetwork(config *config.Config) (*net.IPNet, error) {
	networks, err := GetNetworksFromFilesystem(config)
	if err != nil {
		return nil, err
	}
	liveNetworks, err := GetLiveNetworks()
	if err != nil {
		return nil, err
	}
	nextNetwork, err := GetDefaultPodmanNetwork()
	if err != nil {
		return nil, err
	}
	logrus.Debugf("default network is %s", nextNetwork.String())
	for {
		newNetwork, err := NextSubnet(nextNetwork)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("checking if network %s intersects with other cni networks", nextNetwork.String())
		if intersectsConfig, _ := networkIntersectsWithNetworks(newNetwork, allocatorToIPNets(networks)); intersectsConfig {
			logrus.Debugf("network %s is already being used by a cni configuration", nextNetwork.String())
			nextNetwork = newNetwork
			continue
		}
		logrus.Debugf("checking if network %s intersects with any network interfaces", nextNetwork.String())
		if intersectsLive, _ := networkIntersectsWithNetworks(newNetwork, liveNetworks); !intersectsLive {
			break
		}
		logrus.Debugf("network %s is being used by a network interface", nextNetwork.String())
		nextNetwork = newNetwork
	}
	return nextNetwork, nil
}

func allocatorToIPNets(networks []*allocator.Net) []*net.IPNet {
	var nets []*net.IPNet
	for _, network := range networks {
		if len(network.IPAM.Ranges) > 0 {
			// this is the new IPAM range style
			// append each subnet from ipam the rangeset
			for _, r := range network.IPAM.Ranges[0] {
				nets = append(nets, newIPNetFromSubnet(r.Subnet))
			}
		} else {
			//	 looks like the old, deprecated style
			nets = append(nets, newIPNetFromSubnet(network.IPAM.Subnet))
		}
	}
	return nets
}

func newIPNetFromSubnet(subnet types.IPNet) *net.IPNet {
	n := net.IPNet{
		IP:   subnet.IP,
		Mask: subnet.Mask,
	}
	return &n
}

func networkIntersectsWithNetworks(n *net.IPNet, networklist []*net.IPNet) (bool, *net.IPNet) {
	for _, nw := range networklist {
		if networkIntersect(n, nw) {
			return true, nw
		}
	}
	return false, nil
}

func networkIntersect(n1, n2 *net.IPNet) bool {
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

// ValidateUserNetworkIsAvailable returns via an error if a network is available
// to be used
func ValidateUserNetworkIsAvailable(config *config.Config, userNet *net.IPNet) error {
	if len(userNet.IP) == 0 || len(userNet.Mask) == 0 {
		return errors.Errorf("network %s's ip or mask cannot be empty", userNet.String())
	}

	ones, bit := userNet.Mask.Size()
	if ones == 0 || bit == 0 {
		return errors.Errorf("network %s's mask is invalid", userNet.String())
	}

	networks, err := GetNetworksFromFilesystem(config)
	if err != nil {
		return err
	}
	liveNetworks, err := GetLiveNetworks()
	if err != nil {
		return err
	}
	logrus.Debugf("checking if network %s exists in cni networks", userNet.String())
	if intersectsConfig, _ := networkIntersectsWithNetworks(userNet, allocatorToIPNets(networks)); intersectsConfig {
		return errors.Errorf("network %s is already being used by a cni configuration", userNet.String())
	}
	logrus.Debugf("checking if network %s exists in any network interfaces", userNet.String())
	if intersectsLive, _ := networkIntersectsWithNetworks(userNet, liveNetworks); intersectsLive {
		return errors.Errorf("network %s is being used by a network interface", userNet.String())
	}
	return nil
}

// RemoveNetwork removes a given network by name.  If the network has container associated with it, that
// must be handled outside the context of this.
func RemoveNetwork(config *config.Config, name string) error {
	l, err := acquireCNILock(filepath.Join(config.Engine.TmpDir, LockFileName))
	if err != nil {
		return err
	}
	defer l.releaseCNILock()
	cniPath, err := GetCNIConfigPathByName(config, name)
	if err != nil {
		return err
	}
	// Before we delete the configuration file, we need to make sure we can read and parse
	// it to get the network interface name so we can remove that too
	interfaceName, err := GetInterfaceNameFromConfig(cniPath)
	if err == nil {
		// Don't try to remove the network interface if we are not root
		if !rootless.IsRootless() {
			liveNetworkNames, err := GetLiveNetworkNames()
			if err != nil {
				return errors.Wrapf(err, "failed to get live network names")
			}
			if util.StringInSlice(interfaceName, liveNetworkNames) {
				if err := RemoveInterface(interfaceName); err != nil {
					return errors.Wrapf(err, "failed to delete the network interface %q", interfaceName)
				}
			}
		}
	} else if err != ErrNoSuchNetworkInterface {
		// Don't error if we couldn't find the network interface name
		return err
	}
	// Remove the configuration file
	if err := os.Remove(cniPath); err != nil {
		return errors.Wrap(err, "failed to remove network configuration")
	}
	return nil
}

// InspectNetwork reads a CNI config and returns its configuration
func InspectNetwork(config *config.Config, name string) (map[string]interface{}, error) {
	b, err := ReadRawCNIConfByName(config, name)
	if err != nil {
		return nil, err
	}
	rawList := make(map[string]interface{})
	err = json.Unmarshal(b, &rawList)
	return rawList, err
}

// Exists says whether a given network exists or not; it meant
// specifically for restful responses so 404s can be used
func Exists(config *config.Config, name string) (bool, error) {
	_, err := ReadRawCNIConfByName(config, name)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchNetwork {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
