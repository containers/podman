package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/network"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ErrNoSuchNetworkInterface indicates that no network interface exists
var ErrNoSuchNetworkInterface = errors.New("unable to find interface name for network")

// GetCNIConfDir get CNI configuration directory
func GetCNIConfDir(configArg *config.Config) string {
	if len(configArg.Network.NetworkConfigDir) < 1 {
		dc, err := config.DefaultConfig()
		if err != nil {
			// Fallback to hard-coded dir
			return CNIConfigDir
		}
		return dc.Network.NetworkConfigDir
	}
	return configArg.Network.NetworkConfigDir
}

// LoadCNIConfsFromDir loads all the CNI configurations from a dir
func LoadCNIConfsFromDir(dir string) ([]*libcni.NetworkConfigList, error) {
	files, err := libcni.ConfFiles(dir, []string{".conflist"})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	configs := make([]*libcni.NetworkConfigList, 0, len(files))
	for _, confFile := range files {
		conf, err := libcni.ConfListFromFile(confFile)
		if err != nil {
			return nil, errors.Wrapf(err, "in %s", confFile)
		}
		configs = append(configs, conf)
	}
	return configs, nil
}

// GetCNIConfigPathByNameOrID finds a CNI network by name and
// returns its configuration file path
func GetCNIConfigPathByNameOrID(config *config.Config, name string) (string, error) {
	files, err := libcni.ConfFiles(GetCNIConfDir(config), []string{".conflist"})
	if err != nil {
		return "", err
	}
	idMatch := 0
	file := ""
	for _, confFile := range files {
		conf, err := libcni.ConfListFromFile(confFile)
		if err != nil {
			return "", errors.Wrapf(err, "in %s", confFile)
		}
		if conf.Name == name {
			return confFile, nil
		}
		if strings.HasPrefix(network.GetNetworkID(conf.Name), name) {
			idMatch++
			file = confFile
		}
	}
	if idMatch == 1 {
		return file, nil
	}
	if idMatch > 1 {
		return "", errors.Errorf("more than one result for network ID %s", name)
	}
	return "", errors.Wrap(define.ErrNoSuchNetwork, fmt.Sprintf("unable to find network configuration for %s", name))
}

// ReadRawCNIConfByNameOrID reads the raw CNI configuration for a CNI
// network by name
func ReadRawCNIConfByNameOrID(config *config.Config, name string) ([]byte, error) {
	confFile, err := GetCNIConfigPathByNameOrID(config, name)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(confFile)
	return b, err
}

// GetNetworkLabels returns a list of labels as a string
func GetNetworkLabels(list *libcni.NetworkConfigList) NcLabels {
	cniJSON := make(map[string]interface{})
	err := json.Unmarshal(list.Bytes, &cniJSON)
	if err != nil {
		logrus.Errorf("failed to unmarshal network config %v %v", cniJSON["name"], err)
		return nil
	}
	if args, ok := cniJSON["args"]; ok {
		if key, ok := args.(map[string]interface{}); ok {
			if labels, ok := key[PodmanLabelKey]; ok {
				if labels, ok := labels.(map[string]interface{}); ok {
					result := make(NcLabels, len(labels))
					for k, v := range labels {
						if v, ok := v.(string); ok {
							result[k] = v
						} else {
							logrus.Errorf("network config %v invalid label value type %T should be string", cniJSON["name"], labels)
						}
					}
					return result
				}
				logrus.Errorf("network config %v invalid label type %T should be map[string]string", cniJSON["name"], labels)
			}
		}
	}
	return nil
}

// GetNetworksFromFilesystem gets all the networks from the cni configuration
// files
func GetNetworksFromFilesystem(config *config.Config) ([]*allocator.Net, error) {
	var cniNetworks []*allocator.Net

	networks, err := LoadCNIConfsFromDir(GetCNIConfDir(config))
	if err != nil {
		return nil, err
	}
	for _, n := range networks {
		for _, cniplugin := range n.Plugins {
			if cniplugin.Network.Type == "bridge" {
				ipamConf := allocator.Net{}
				if err := json.Unmarshal(cniplugin.Bytes, &ipamConf); err != nil {
					return nil, err
				}
				cniNetworks = append(cniNetworks, &ipamConf)
				break
			}
		}
	}
	return cniNetworks, nil
}

// GetNetworkNamesFromFileSystem gets all the names from the cni network
// configuration files
func GetNetworkNamesFromFileSystem(config *config.Config) ([]string, error) {
	networks, err := LoadCNIConfsFromDir(GetCNIConfDir(config))
	if err != nil {
		return nil, err
	}
	networkNames := []string{}
	for _, n := range networks {
		networkNames = append(networkNames, n.Name)
	}
	return networkNames, nil
}

// GetInterfaceNameFromConfig returns the interface name for the bridge plugin
func GetInterfaceNameFromConfig(path string) (string, error) {
	var name string
	conf, err := libcni.ConfListFromFile(path)
	if err != nil {
		return "", err
	}
	for _, cniplugin := range conf.Plugins {
		if cniplugin.Network.Type == "bridge" {
			plugin := make(map[string]interface{})
			if err := json.Unmarshal(cniplugin.Bytes, &plugin); err != nil {
				return "", err
			}
			name = plugin["bridge"].(string)
			break
		}
	}
	if len(name) == 0 {
		return "", ErrNoSuchNetworkInterface
	}
	return name, nil
}

// GetBridgeNamesFromFileSystem is a convenience function to get all the bridge
// names from the configured networks
func GetBridgeNamesFromFileSystem(config *config.Config) ([]string, error) {
	networks, err := LoadCNIConfsFromDir(GetCNIConfDir(config))
	if err != nil {
		return nil, err
	}

	bridgeNames := []string{}
	for _, n := range networks {
		var name string
		// iterate network conflists
		for _, cniplugin := range n.Plugins {
			// iterate plugins
			if cniplugin.Network.Type == "bridge" {
				plugin := make(map[string]interface{})
				if err := json.Unmarshal(cniplugin.Bytes, &plugin); err != nil {
					continue
				}
				name = plugin["bridge"].(string)
			}
		}
		bridgeNames = append(bridgeNames, name)
	}
	return bridgeNames, nil
}
