// +build linux

package cni

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/containernetworking/cni/libcni"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// Setup will setup the container network namespace. It returns
// a map of StatusBlocks, the key is the network name.
func (n *cniNetwork) Setup(namespacePath string, options types.SetupOptions) (map[string]types.StatusBlock, error) {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return nil, err
	}

	if namespacePath == "" {
		return nil, errors.New("namespacePath is empty")
	}
	if options.ContainerID == "" {
		return nil, errors.New("ContainerID is empty")
	}
	if len(options.Networks) == 0 {
		return nil, errors.New("must specify at least one network")
	}
	for name, netOpts := range options.Networks {
		network := n.networks[name]
		if network == nil {
			return nil, errors.Wrapf(define.ErrNoSuchNetwork, "network %s", name)
		}
		err := validatePerNetworkOpts(network, netOpts)
		if err != nil {
			return nil, err
		}
	}

	// set the loopback adapter up in the container netns
	err = ns.WithNetNSPath(namespacePath, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName("lo")
		if err == nil {
			err = netlink.LinkSetUp(link)
		}
		return err
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set the loopback adapter up")
	}

	var retErr error
	teardownOpts := options
	teardownOpts.Networks = map[string]types.PerNetworkOptions{}
	// make sure to teardown the already connected networks on error
	defer func() {
		if retErr != nil {
			if len(teardownOpts.Networks) > 0 {
				err := n.teardown(namespacePath, types.TeardownOptions(teardownOpts))
				if err != nil {
					logrus.Warn(err)
				}
			}
		}
	}()

	ports, err := convertSpecgenPortsToCNIPorts(options.PortMappings)
	if err != nil {
		return nil, err
	}

	results := make(map[string]types.StatusBlock, len(options.Networks))
	for name, netOpts := range options.Networks {
		network := n.networks[name]
		rt := getRuntimeConfig(namespacePath, options.ContainerName, options.ContainerID, name, ports, netOpts)

		// If we have more than one static ip we need parse the ips via runtime config,
		// make sure to add the ips capability to the first plugin otherwise it doesn't get the ips
		if len(netOpts.StaticIPs) > 0 && !network.cniNet.Plugins[0].Network.Capabilities["ips"] {
			caps := make(map[string]interface{})
			caps["capabilities"] = map[string]bool{"ips": true}
			network.cniNet.Plugins[0], retErr = libcni.InjectConf(network.cniNet.Plugins[0], caps)
			if retErr != nil {
				return nil, retErr
			}
		}

		var res cnitypes.Result
		res, retErr = n.cniConf.AddNetworkList(context.Background(), network.cniNet, rt)
		// Add this network to teardown opts since it is now connected.
		// Also add this if an errors was returned since we want to call teardown on this regardless.
		teardownOpts.Networks[name] = netOpts
		if retErr != nil {
			return nil, retErr
		}

		var cnires *current.Result
		cnires, retErr = current.GetResult(res)
		if retErr != nil {
			return nil, retErr
		}
		logrus.Debugf("cni result for container %s network %s: %v", options.ContainerID, name, cnires)
		var status types.StatusBlock
		status, retErr = cniResultToStatus(cnires)
		if retErr != nil {
			return nil, retErr
		}
		results[name] = status
	}
	return results, nil
}

// cniResultToStatus convert the cni result to status block
func cniResultToStatus(cniResult *current.Result) (types.StatusBlock, error) {
	result := types.StatusBlock{}
	nameservers := make([]net.IP, 0, len(cniResult.DNS.Nameservers))
	for _, nameserver := range cniResult.DNS.Nameservers {
		ip := net.ParseIP(nameserver)
		if ip == nil {
			return result, errors.Errorf("failed to parse cni nameserver ip %s", nameserver)
		}
		nameservers = append(nameservers, ip)
	}
	result.DNSServerIPs = nameservers
	result.DNSSearchDomains = cniResult.DNS.Search

	interfaces := make(map[string]types.NetInterface)
	for _, ip := range cniResult.IPs {
		if ip.Interface == nil {
			// we do no expect ips without an interface
			continue
		}
		if len(cniResult.Interfaces) <= *ip.Interface {
			return result, errors.Errorf("invalid cni result, interface index %d out of range", *ip.Interface)
		}
		cniInt := cniResult.Interfaces[*ip.Interface]
		netInt, ok := interfaces[cniInt.Name]
		if ok {
			netInt.Networks = append(netInt.Networks, types.NetAddress{
				Subnet:  types.IPNet{IPNet: ip.Address},
				Gateway: ip.Gateway,
			})
			interfaces[cniInt.Name] = netInt
		} else {
			mac, err := net.ParseMAC(cniInt.Mac)
			if err != nil {
				return result, err
			}
			interfaces[cniInt.Name] = types.NetInterface{
				MacAddress: mac,
				Networks: []types.NetAddress{{
					Subnet:  types.IPNet{IPNet: ip.Address},
					Gateway: ip.Gateway,
				}},
			}
		}
	}
	result.Interfaces = interfaces
	return result, nil
}

// validatePerNetworkOpts checks that all given static ips are in a subnet on this network
func validatePerNetworkOpts(network *network, netOpts types.PerNetworkOptions) error {
	if netOpts.InterfaceName == "" {
		return errors.Errorf("interface name on network %s is empty", network.libpodNet.Name)
	}
outer:
	for _, ip := range netOpts.StaticIPs {
		for _, s := range network.libpodNet.Subnets {
			if s.Subnet.Contains(ip) {
				continue outer
			}
		}
		return errors.Errorf("requested static ip %s not in any subnet on network %s", ip.String(), network.libpodNet.Name)
	}
	if len(netOpts.Aliases) > 0 && !network.libpodNet.DNSEnabled {
		return errors.New("cannot set aliases on a network without dns enabled")
	}
	return nil
}

func getRuntimeConfig(netns, conName, conID, networkName string, ports []cniPortMapEntry, opts types.PerNetworkOptions) *libcni.RuntimeConf {
	rt := &libcni.RuntimeConf{
		ContainerID: conID,
		NetNS:       netns,
		IfName:      opts.InterfaceName,
		Args: [][2]string{
			{"IgnoreUnknown", "1"},
			// Do not set the K8S env vars, see https://github.com/containers/podman/issues/12083.
			// Only K8S_POD_NAME is used by dnsname to get the container name.
			{"K8S_POD_NAME", conName},
		},
		CapabilityArgs: map[string]interface{}{},
	}

	// Propagate environment CNI_ARGS
	for _, kvpairs := range strings.Split(os.Getenv("CNI_ARGS"), ";") {
		if keyval := strings.SplitN(kvpairs, "=", 2); len(keyval) == 2 {
			rt.Args = append(rt.Args, [2]string{keyval[0], keyval[1]})
		}
	}

	// Add mac address to cni args
	if len(opts.StaticMAC) > 0 {
		rt.Args = append(rt.Args, [2]string{"MAC", opts.StaticMAC.String()})
	}

	if len(opts.StaticIPs) == 1 {
		// Add a single IP to the args field. CNI plugins < 1.0.0
		// do not support multiple ips via capability args.
		rt.Args = append(rt.Args, [2]string{"IP", opts.StaticIPs[0].String()})
	} else if len(opts.StaticIPs) > 1 {
		// Set the static ips in the capability args
		// to support more than one static ip per network.
		rt.CapabilityArgs["ips"] = opts.StaticIPs
	}

	// Set network aliases for the dnsname plugin.
	if len(opts.Aliases) > 0 {
		rt.CapabilityArgs["aliases"] = map[string][]string{
			networkName: opts.Aliases,
		}
	}

	// Set PortMappings in Capabilities
	if len(ports) > 0 {
		rt.CapabilityArgs["portMappings"] = ports
	}

	return rt
}

// Teardown will teardown the container network namespace.
func (n *cniNetwork) Teardown(namespacePath string, options types.TeardownOptions) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	err := n.loadNetworks()
	if err != nil {
		return err
	}
	return n.teardown(namespacePath, options)
}

func (n *cniNetwork) teardown(namespacePath string, options types.TeardownOptions) error {
	// Note: An empty namespacePath is allowed because some plugins
	// still need teardown, for example ipam should remove used ip allocations.

	ports, err := convertSpecgenPortsToCNIPorts(options.PortMappings)
	if err != nil {
		return err
	}

	var multiErr *multierror.Error
	for name, netOpts := range options.Networks {
		rt := getRuntimeConfig(namespacePath, options.ContainerName, options.ContainerID, name, ports, netOpts)

		cniConfList, newRt, err := getCachedNetworkConfig(n.cniConf, name, rt)
		if err == nil {
			rt = newRt
		} else {
			logrus.Warnf("failed to load cached network config: %v, falling back to loading network %s from disk", err, name)
			network := n.networks[name]
			if network == nil {
				multiErr = multierror.Append(multiErr, errors.Wrapf(define.ErrNoSuchNetwork, "network %s", name))
				continue
			}
			cniConfList = network.cniNet
		}

		err = n.cniConf.DelNetworkList(context.Background(), cniConfList, rt)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return multiErr.ErrorOrNil()
}

func getCachedNetworkConfig(cniConf *libcni.CNIConfig, name string, rt *libcni.RuntimeConf) (*libcni.NetworkConfigList, *libcni.RuntimeConf, error) {
	cniConfList := &libcni.NetworkConfigList{
		Name: name,
	}
	confBytes, rt, err := cniConf.GetNetworkListCachedConfig(cniConfList, rt)
	if err != nil {
		return nil, nil, err
	} else if confBytes == nil {
		return nil, nil, errors.Errorf("network %s not found in CNI cache", name)
	}

	cniConfList, err = libcni.ConfListFromBytes(confBytes)
	if err != nil {
		return nil, nil, err
	}
	return cniConfList, rt, nil
}
