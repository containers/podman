package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/version"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
)

func Create(name string, options entities.NetworkCreateOptions, r *libpod.Runtime) (*entities.NetworkCreateReport, error) {
	var fileName string
	if err := isSupportedDriver(options.Driver); err != nil {
		return nil, err
	}
	config, err := r.GetConfig()
	if err != nil {
		return nil, err
	}
	// Acquire a lock for CNI
	l, err := acquireCNILock(filepath.Join(config.Engine.TmpDir, LockFileName))
	if err != nil {
		return nil, err
	}
	defer l.releaseCNILock()
	if len(options.MacVLAN) > 0 {
		fileName, err = createMacVLAN(r, name, options)
	} else {
		fileName, err = createBridge(r, name, options)
	}
	if err != nil {
		return nil, err
	}
	return &entities.NetworkCreateReport{Filename: fileName}, nil
}

// createBridge creates a CNI network
func createBridge(r *libpod.Runtime, name string, options entities.NetworkCreateOptions) (string, error) {
	isGateway := true
	ipMasq := true
	subnet := &options.Subnet
	ipRange := options.Range
	runtimeConfig, err := r.GetConfig()
	if err != nil {
		return "", err
	}
	// if range is provided, make sure it is "in" network
	if subnet.IP != nil {
		// if network is provided, does it conflict with existing CNI or live networks
		err = ValidateUserNetworkIsAvailable(runtimeConfig, subnet)
	} else {
		// if no network is provided, figure out network
		subnet, err = GetFreeNetwork(runtimeConfig)
	}
	if err != nil {
		return "", err
	}
	gateway := options.Gateway
	if gateway == nil {
		// if no gateway is provided, provide it as first ip of network
		gateway = CalcGatewayIP(subnet)
	}
	// if network is provided and if gateway is provided, make sure it is "in" network
	if options.Subnet.IP != nil && options.Gateway != nil {
		if !subnet.Contains(gateway) {
			return "", errors.Errorf("gateway %s is not in valid for subnet %s", gateway.String(), subnet.String())
		}
	}
	if options.Internal {
		isGateway = false
		ipMasq = false
	}

	// if a range is given, we need to ensure it is "in" the network range.
	if options.Range.IP != nil {
		if options.Subnet.IP == nil {
			return "", errors.New("you must define a subnet range to define an ip-range")
		}
		firstIP, err := FirstIPInSubnet(&options.Range)
		if err != nil {
			return "", err
		}
		lastIP, err := LastIPInSubnet(&options.Range)
		if err != nil {
			return "", err
		}
		if !subnet.Contains(firstIP) || !subnet.Contains(lastIP) {
			return "", errors.Errorf("the ip range %s does not fall within the subnet range %s", options.Range.String(), subnet.String())
		}
	}
	bridgeDeviceName, err := GetFreeDeviceName(runtimeConfig)
	if err != nil {
		return "", err
	}

	if len(name) > 0 {
		netNames, err := GetNetworkNamesFromFileSystem(runtimeConfig)
		if err != nil {
			return "", err
		}
		if util.StringInSlice(name, netNames) {
			return "", errors.Errorf("the network name %s is already used", name)
		}
	} else {
		// If no name is given, we give the name of the bridge device
		name = bridgeDeviceName
	}

	ncList := NewNcList(name, version.Current())
	var plugins []CNIPlugins
	var routes []IPAMRoute

	defaultRoute, err := NewIPAMDefaultRoute(IsIPv6(subnet.IP))
	if err != nil {
		return "", err
	}
	routes = append(routes, defaultRoute)
	ipamConfig, err := NewIPAMHostLocalConf(subnet, routes, ipRange, gateway)
	if err != nil {
		return "", err
	}

	// TODO need to iron out the role of isDefaultGW and IPMasq
	bridge := NewHostLocalBridge(bridgeDeviceName, isGateway, false, ipMasq, ipamConfig)
	plugins = append(plugins, bridge)
	plugins = append(plugins, NewPortMapPlugin())
	plugins = append(plugins, NewFirewallPlugin())
	// if we find the dnsname plugin or are rootless, we add configuration for it
	// the rootless-cni-infra container has the dnsname plugin always installed
	if (HasDNSNamePlugin(runtimeConfig.Network.CNIPluginDirs) || rootless.IsRootless()) && !options.DisableDNS {
		// Note: in the future we might like to allow for dynamic domain names
		plugins = append(plugins, NewDNSNamePlugin(DefaultPodmanDomainName))
	}
	ncList["plugins"] = plugins
	b, err := json.MarshalIndent(ncList, "", "   ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(GetCNIConfDir(runtimeConfig), 0755); err != nil {
		return "", err
	}
	cniPathName := filepath.Join(GetCNIConfDir(runtimeConfig), fmt.Sprintf("%s.conflist", name))
	err = ioutil.WriteFile(cniPathName, b, 0644)
	return cniPathName, err
}

func createMacVLAN(r *libpod.Runtime, name string, options entities.NetworkCreateOptions) (string, error) {
	var (
		plugins []CNIPlugins
	)
	liveNetNames, err := GetLiveNetworkNames()
	if err != nil {
		return "", err
	}

	config, err := r.GetConfig()
	if err != nil {
		return "", err
	}

	// Make sure the host-device exists
	if !util.StringInSlice(options.MacVLAN, liveNetNames) {
		return "", errors.Errorf("failed to find network interface %q", options.MacVLAN)
	}
	if len(name) > 0 {
		netNames, err := GetNetworkNamesFromFileSystem(config)
		if err != nil {
			return "", err
		}
		if util.StringInSlice(name, netNames) {
			return "", errors.Errorf("the network name %s is already used", name)
		}
	} else {
		name, err = GetFreeDeviceName(config)
		if err != nil {
			return "", err
		}
	}
	ncList := NewNcList(name, version.Current())
	macvlan := NewMacVLANPlugin(options.MacVLAN)
	plugins = append(plugins, macvlan)
	ncList["plugins"] = plugins
	b, err := json.MarshalIndent(ncList, "", "   ")
	if err != nil {
		return "", err
	}
	cniPathName := filepath.Join(GetCNIConfDir(config), fmt.Sprintf("%s.conflist", name))
	err = ioutil.WriteFile(cniPathName, b, 0644)
	return cniPathName, err
}
