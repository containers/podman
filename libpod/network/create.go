package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containernetworking/cni/pkg/version"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Create the CNI network
func Create(name string, options entities.NetworkCreateOptions, runtimeConfig *config.Config) (*entities.NetworkCreateReport, error) {
	var fileName string
	if err := isSupportedDriver(options.Driver); err != nil {
		return nil, err
	}
	// Acquire a lock for CNI
	l, err := acquireCNILock(runtimeConfig)
	if err != nil {
		return nil, err
	}
	defer l.releaseCNILock()
	if len(options.MacVLAN) > 0 || options.Driver == MacVLANNetworkDriver {
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

// parseMTU parses the mtu option
func parseMTU(mtu string) (int, error) {
	if mtu == "" {
		return 0, nil // default
	}
	m, err := strconv.Atoi(mtu)
	if err != nil {
		return 0, err
	}
	if m < 0 {
		return 0, errors.Errorf("the value %d for mtu is less than zero", m)
	}
	return m, nil
}

// parseVlan parses the vlan option
func parseVlan(vlan string) (int, error) {
	if vlan == "" {
		return 0, nil // default
	}
	return strconv.Atoi(vlan)
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

	var mtu int
	var vlan int
	for k, v := range options.Options {
		var err error
		switch k {
		case "mtu":
			mtu, err = parseMTU(v)
			if err != nil {
				return "", err
			}

		case "vlan":
			vlan, err = parseVlan(v)
			if err != nil {
				return "", err
			}

		default:
			return "", errors.Errorf("unsupported option %s", k)
		}
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
	ncList := NewNcList(name, version.Current(), options.Labels)
	var plugins []CNIPlugins
	// TODO need to iron out the role of isDefaultGW and IPMasq
	bridge := NewHostLocalBridge(bridgeDeviceName, isGateway, false, ipMasq, mtu, vlan, ipamConfig)
	plugins = append(plugins, bridge)
	plugins = append(plugins, NewPortMapPlugin())
	plugins = append(plugins, NewFirewallPlugin())
	plugins = append(plugins, NewTuningPlugin())
	// if we find the dnsname plugin we add configuration for it
	if HasDNSNamePlugin(runtimeConfig.Network.CNIPluginDirs) && !options.DisableDNS {
		if options.Internal {
			logrus.Warnf("dnsname and --internal networks are incompatible.  dnsname plugin not configured for network %s", name)
		} else {
			// Note: in the future we might like to allow for dynamic domain names
			plugins = append(plugins, NewDNSNamePlugin(DefaultPodmanDomainName))
		}
	}
	// Add the podman-machine CNI plugin if we are in a machine
	if runtimeConfig.MachineEnabled() { // check if we are in a machine vm
		plugins = append(plugins, NewPodmanMachinePlugin())
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
		mtu     int
		plugins []CNIPlugins
	)
	liveNetNames, err := GetLiveNetworkNames()
	if err != nil {
		return "", err
	}

	// The parent can be defined with --macvlan or as an option (-o parent:device)
	parentNetworkDevice := options.MacVLAN
	if len(parentNetworkDevice) < 1 {
		if parent, ok := options.Options["parent"]; ok {
			parentNetworkDevice = parent
		}
	}

	// Make sure the host-device exists if provided
	if len(parentNetworkDevice) > 0 && !util.StringInSlice(parentNetworkDevice, liveNetNames) {
		return "", errors.Errorf("failed to find network interface %q", parentNetworkDevice)
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
	ncList := NewNcList(name, version.Current(), options.Labels)
	if val, ok := options.Options["mtu"]; ok {
		intVal, err := strconv.Atoi(val)
		if err != nil {
			return "", err
		}
		if intVal > 0 {
			mtu = intVal
		}
	}
	macvlan, err := NewMacVLANPlugin(parentNetworkDevice, options.Gateway, &options.Range, &options.Subnet, mtu)
	if err != nil {
		return "", err
	}
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
