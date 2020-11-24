package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/version"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
)

// Create the CNI network
func Create(name string, options entities.NetworkCreateOptions, runtimeConfig *config.Config) (*entities.NetworkCreateReport, error) {
	var fileName string
	if err := isSupportedDriver(options.Driver); err != nil {
		return nil, err
	}
	// Acquire a lock for CNI
	l, err := acquireCNILock(filepath.Join(runtimeConfig.Engine.TmpDir, LockFileName))
	if err != nil {
		return nil, err
	}
	defer l.releaseCNILock()
	if len(options.MacVLAN) > 0 {
		fileName, err = createMacVLAN(name, options, runtimeConfig)
	} else {
		fileName, err = createBridge(name, options, runtimeConfig)
	}
	if err != nil {
		return nil, err
	}
	return &entities.NetworkCreateReport{Filename: fileName}, nil
}

// validateBridgeOptions validate the bridge networking options
func validateBridgeOptions(options entities.NetworkCreateOptions) error {
	subnet := &options.Subnet
	ipRange := &options.Range
	gateway := options.Gateway
	// if IPv6 is set an IPv6 subnet MUST be specified
	if options.IPv6 && ((subnet.IP == nil) || (subnet.IP != nil && !IsIPv6(subnet.IP))) {
		return errors.Errorf("ipv6 option requires an IPv6 --subnet to be provided")
	}
	// range and gateway depend on subnet
	if subnet.IP == nil && (ipRange.IP != nil || gateway != nil) {
		return errors.Errorf("every ip-range or gateway must have a corresponding subnet")
	}

	// if a range is given, we need to ensure it is "in" the network range.
	if ipRange.IP != nil {
		firstIP, err := FirstIPInSubnet(ipRange)
		if err != nil {
			return errors.Wrapf(err, "failed to get first IP address from ip-range")
		}
		lastIP, err := LastIPInSubnet(ipRange)
		if err != nil {
			return errors.Wrapf(err, "failed to get last IP address from ip-range")
		}
		if !subnet.Contains(firstIP) || !subnet.Contains(lastIP) {
			return errors.Errorf("the ip range %s does not fall within the subnet range %s", ipRange.String(), subnet.String())
		}
	}

	// if network is provided and if gateway is provided, make sure it is "in" network
	if gateway != nil && !subnet.Contains(gateway) {
		return errors.Errorf("gateway %s is not in valid for subnet %s", gateway.String(), subnet.String())
	}

	return nil

}

// createBridge creates a CNI network
func createBridge(name string, options entities.NetworkCreateOptions, runtimeConfig *config.Config) (string, error) {
	var (
		ipamRanges [][]IPAMLocalHostRangeConf
		err        error
		routes     []IPAMRoute
	)
	isGateway := true
	ipMasq := true

	// validate options
	if err := validateBridgeOptions(options); err != nil {
		return "", err
	}

	// For compatibility with the docker implementation:
	// if IPv6 is enabled (it really means dual-stack) then an IPv6 subnet has to be provided, and one free network is allocated for IPv4
	// if IPv6 is not specified the subnet may be specified and can be either IPv4 or IPv6 (podman, unlike docker, allows IPv6 only networks)
	// If not subnet is specified an IPv4 subnet will be allocated
	subnet := &options.Subnet
	ipRange := &options.Range
	gateway := options.Gateway
	if subnet.IP != nil {
		// if network is provided, does it conflict with existing CNI or live networks
		err = ValidateUserNetworkIsAvailable(runtimeConfig, subnet)
		if err != nil {
			return "", err
		}
		// obtain CNI subnet default route
		defaultRoute, err := NewIPAMDefaultRoute(IsIPv6(subnet.IP))
		if err != nil {
			return "", err
		}
		routes = append(routes, defaultRoute)
		// obtain CNI range
		ipamRange, err := NewIPAMLocalHostRange(subnet, ipRange, gateway)
		if err != nil {
			return "", err
		}
		ipamRanges = append(ipamRanges, ipamRange)
	}
	// if no network is provided or IPv6 flag used, figure out the IPv4 network
	if options.IPv6 || len(routes) == 0 {
		subnetV4, err := GetFreeNetwork(runtimeConfig)
		if err != nil {
			return "", err
		}
		// obtain IPv4 default route
		defaultRoute, err := NewIPAMDefaultRoute(false)
		if err != nil {
			return "", err
		}
		routes = append(routes, defaultRoute)
		// the CNI bridge plugin does not need to set
		// the range or gateway options explicitly
		ipamRange, err := NewIPAMLocalHostRange(subnetV4, nil, nil)
		if err != nil {
			return "", err
		}
		ipamRanges = append(ipamRanges, ipamRange)
	}

	// create CNI config
	ipamConfig, err := NewIPAMHostLocalConf(routes, ipamRanges)
	if err != nil {
		return "", err
	}

	if options.Internal {
		isGateway = false
		ipMasq = false
	}

	// obtain host bridge name
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

	// create CNI plugin configuration
	ncList := NewNcList(name, version.Current())
	var plugins []CNIPlugins
	// TODO need to iron out the role of isDefaultGW and IPMasq
	bridge := NewHostLocalBridge(bridgeDeviceName, isGateway, false, ipMasq, ipamConfig)
	plugins = append(plugins, bridge)
	plugins = append(plugins, NewPortMapPlugin())
	plugins = append(plugins, NewFirewallPlugin())
	plugins = append(plugins, NewTuningPlugin())
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

func createMacVLAN(name string, options entities.NetworkCreateOptions, runtimeConfig *config.Config) (string, error) {
	var (
		plugins []CNIPlugins
	)
	liveNetNames, err := GetLiveNetworkNames()
	if err != nil {
		return "", err
	}

	// Make sure the host-device exists
	if !util.StringInSlice(options.MacVLAN, liveNetNames) {
		return "", errors.Errorf("failed to find network interface %q", options.MacVLAN)
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
		name, err = GetFreeDeviceName(runtimeConfig)
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
	cniPathName := filepath.Join(GetCNIConfDir(runtimeConfig), fmt.Sprintf("%s.conflist", name))
	err = ioutil.WriteFile(cniPathName, b, 0644)
	return cniPathName, err
}
