// +build !remoteclient

package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/tabwriter"

	cniversion "github.com/containernetworking/cni/pkg/version"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/network"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
)

func getCNIConfDir(r *LocalRuntime) (string, error) {
	config, err := r.GetConfig()
	if err != nil {
		return "", err
	}
	configPath := config.NetworkConfigDir

	if len(config.NetworkConfigDir) < 1 {
		configPath = network.CNIConfigDir
	}
	return configPath, nil
}

// NetworkList displays summary information about CNI networks
func (r *LocalRuntime) NetworkList(cli *cliconfig.NetworkListValues) error {
	cniConfigPath, err := getCNIConfDir(r)
	if err != nil {
		return err
	}
	networks, err := network.LoadCNIConfsFromDir(cniConfigPath)
	if err != nil {
		return err
	}
	// quiet means we only print the network names
	if cli.Quiet {
		for _, cniNetwork := range networks {
			fmt.Println(cniNetwork.Name)
		}
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tVERSION\tPLUGINS"); err != nil {
		return err
	}
	for _, cniNetwork := range networks {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", cniNetwork.Name, cniNetwork.CNIVersion, network.GetCNIPlugins(cniNetwork)); err != nil {
			return err
		}
	}
	return w.Flush()
}

// NetworkInspect displays the raw CNI configuration for one
// or more CNI networks
func (r *LocalRuntime) NetworkInspect(cli *cliconfig.NetworkInspectValues) error {
	var (
		rawCNINetworks []map[string]interface{}
	)
	for _, name := range cli.InputArgs {
		b, err := network.ReadRawCNIConfByName(name)
		if err != nil {
			return err
		}
		rawList := make(map[string]interface{})
		if err := json.Unmarshal(b, &rawList); err != nil {
			return fmt.Errorf("error parsing configuration list: %s", err)
		}
		rawCNINetworks = append(rawCNINetworks, rawList)
	}
	out, err := json.MarshalIndent(rawCNINetworks, "", "\t")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", out)
	return nil
}

// NetworkRemove deletes one or more CNI networks
func (r *LocalRuntime) NetworkRemove(ctx context.Context, cli *cliconfig.NetworkRmValues) ([]string, map[string]error, error) {
	var (
		networkRmSuccesses []string
		lastError          error
	)
	networkRmErrors := make(map[string]error)

	for _, name := range cli.InputArgs {
		containers, err := r.GetAllContainers()
		if err != nil {
			return networkRmSuccesses, networkRmErrors, err
		}
		if err := r.removeNetwork(ctx, name, containers, cli.Force); err != nil {
			if lastError != nil {
				networkRmErrors[name] = lastError
			}
			lastError = err
		} else {
			networkRmSuccesses = append(networkRmSuccesses, fmt.Sprintf("Deleted: %s\n", name))
		}
	}
	return networkRmSuccesses, networkRmErrors, lastError
}

// removeNetwork removes a single network and its containers given a force bool
func (r *LocalRuntime) removeNetwork(ctx context.Context, name string, containers []*Container, force bool) error {
	cniPath, err := network.GetCNIConfigPathByName(name)
	if err != nil {
		return err
	}
	// We need to iterate containers looking to see if they belong to the given network
	for _, c := range containers {
		if util.StringInSlice(name, c.Config().Networks) {
			// if user passes force, we nuke containers
			if force {
				if err := r.RemoveContainer(ctx, c.Container, true, true); err != nil {
					return err
				}
			} else {
				// Without the the force option, we return an error
				return errors.Errorf("%q has associated containers with it. use -f to forcibly delete containers", name)
			}

		}
	}
	// Before we delete the configuration file, we need to make sure we can read and parse
	// it to get the network interface name so we can remove that too
	interfaceName, err := network.GetInterfaceNameFromConfig(cniPath)
	if err != nil {
		return errors.Wrapf(err, "failed to find network interface name in %q", cniPath)
	}
	liveNetworkNames, err := network.GetLiveNetworkNames()
	if err != nil {
		return errors.Wrapf(err, "failed to get live network names")
	}
	if util.StringInSlice(interfaceName, liveNetworkNames) {
		if err := network.RemoveInterface(interfaceName); err != nil {
			return errors.Wrapf(err, "failed to delete the network interface %q", interfaceName)
		}
	}
	// Remove the configuration file
	if err := os.Remove(cniPath); err != nil {
		return errors.Wrapf(err, "failed to remove network configuration file %q", cniPath)
	}
	return nil
}

// NetworkCreate creates a CNI network
func (r *LocalRuntime) NetworkCreate(cli *cliconfig.NetworkCreateValues) (string, error) {
	isGateway := true
	ipMasq := true
	subnet := &cli.Network
	ipRange := cli.IPRange
	runtimeConfig, err := r.GetConfig()
	if err != nil {
		return "", err
	}
	// if range is provided, make sure it is "in" network
	if cli.IsSet("subnet") {
		// if network is provided, does it conflict with existing CNI or live networks
		err = network.ValidateUserNetworkIsAvailable(subnet)
	} else {
		// if no network is provided, figure out network
		subnet, err = network.GetFreeNetwork()
	}
	if err != nil {
		return "", err
	}

	gateway := cli.Gateway
	if gateway == nil {
		// if no gateway is provided, provide it as first ip of network
		gateway = network.CalcGatewayIP(subnet)
	}
	// if network is provided and if gateway is provided, make sure it is "in" network
	if cli.IsSet("subnet") && cli.IsSet("gateway") {
		if !subnet.Contains(gateway) {
			return "", errors.Errorf("gateway %s is not in valid for subnet %s", gateway.String(), subnet.String())
		}
	}
	if cli.Internal {
		isGateway = false
		ipMasq = false
	}

	// if a range is given, we need to ensure it is "in" the network range.
	if cli.IsSet("ip-range") {
		if !cli.IsSet("subnet") {
			return "", errors.New("you must define a subnet range to define an ip-range")
		}
		firstIP, err := network.FirstIPInSubnet(&cli.IPRange)
		if err != nil {
			return "", err
		}
		lastIP, err := network.LastIPInSubnet(&cli.IPRange)
		if err != nil {
			return "", err
		}
		if !subnet.Contains(firstIP) || !subnet.Contains(lastIP) {
			return "", errors.Errorf("the ip range %s does not fall within the subnet range %s", cli.IPRange.String(), subnet.String())
		}
	}
	bridgeDeviceName, err := network.GetFreeDeviceName()
	if err != nil {
		return "", err
	}
	// If no name is given, we give the name of the bridge device
	name := bridgeDeviceName
	if len(cli.InputArgs) > 0 {
		name = cli.InputArgs[0]
		netNames, err := network.GetNetworkNamesFromFileSystem()
		if err != nil {
			return "", err
		}
		if util.StringInSlice(name, netNames) {
			return "", errors.Errorf("the network name %s is already used", name)
		}
	}

	ncList := network.NewNcList(name, cniversion.Current())
	var plugins []network.CNIPlugins
	var routes []network.IPAMRoute

	defaultRoute, err := network.NewIPAMDefaultRoute()
	if err != nil {
		return "", err
	}
	routes = append(routes, defaultRoute)
	ipamConfig, err := network.NewIPAMHostLocalConf(subnet, routes, ipRange, gateway)
	if err != nil {
		return "", err
	}

	// TODO need to iron out the role of isDefaultGW and IPMasq
	bridge := network.NewHostLocalBridge(bridgeDeviceName, isGateway, false, ipMasq, ipamConfig)
	plugins = append(plugins, bridge)
	plugins = append(plugins, network.NewPortMapPlugin())
	plugins = append(plugins, network.NewFirewallPlugin())
	// if we find the dnsname plugin, we add configuration for it
	if network.HasDNSNamePlugin(runtimeConfig.CNIPluginDirs) && !cli.DisableDNS {
		// Note: in the future we might like to allow for dynamic domain names
		plugins = append(plugins, network.NewDNSNamePlugin(network.DefaultPodmanDomainName))
	}
	ncList["plugins"] = plugins
	b, err := json.MarshalIndent(ncList, "", "   ")
	if err != nil {
		return "", err
	}
	cniConfigPath, err := getCNIConfDir(r)
	if err != nil {
		return "", err
	}
	cniPathName := filepath.Join(cniConfigPath, fmt.Sprintf("%s.conflist", name))
	err = ioutil.WriteFile(cniPathName, b, 0644)
	return cniPathName, err
}
