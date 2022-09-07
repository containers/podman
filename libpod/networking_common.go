//go:build linux || freebsd
// +build linux freebsd

package libpod

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/machine"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/namespaces"
	"github.com/sirupsen/logrus"
)

// convertPortMappings will remove the HostIP part from the ports when running inside podman machine.
// This is need because a HostIP of 127.0.0.1 would now allow the gvproxy forwarder to reach to open ports.
// For machine the HostIP must only be used by gvproxy and never in the VM.
func (c *Container) convertPortMappings() []types.PortMapping {
	if !machine.IsGvProxyBased() || len(c.config.PortMappings) == 0 {
		return c.config.PortMappings
	}
	// if we run in a machine VM we have to ignore the host IP part
	newPorts := make([]types.PortMapping, 0, len(c.config.PortMappings))
	for _, port := range c.config.PortMappings {
		port.HostIP = ""
		newPorts = append(newPorts, port)
	}
	return newPorts
}

func (c *Container) getNetworkOptions(networkOpts map[string]types.PerNetworkOptions) types.NetworkOptions {
	opts := types.NetworkOptions{
		ContainerID:   c.config.ID,
		ContainerName: getCNIPodName(c),
	}
	opts.PortMappings = c.convertPortMappings()

	// If the container requested special network options use this instead of the config.
	// This is the case for container restore or network reload.
	if c.perNetworkOpts != nil {
		opts.Networks = c.perNetworkOpts
	} else {
		opts.Networks = networkOpts
	}
	return opts
}

// setUpNetwork will set up the the networks, on error it will also tear down the cni
// networks. If rootless it will join/create the rootless network namespace.
func (r *Runtime) setUpNetwork(ns string, opts types.NetworkOptions) (map[string]types.StatusBlock, error) {
	rootlessNetNS, err := r.GetRootlessNetNs(true)
	if err != nil {
		return nil, err
	}
	var results map[string]types.StatusBlock
	setUpPod := func() error {
		results, err = r.network.Setup(ns, types.SetupOptions{NetworkOptions: opts})
		return err
	}
	// rootlessNetNS is nil if we are root
	if rootlessNetNS != nil {
		// execute the setup in the rootless net ns
		err = rootlessNetNS.Do(setUpPod)
		rootlessNetNS.Lock.Unlock()
	} else {
		err = setUpPod()
	}
	return results, err
}

// getCNIPodName return the pod name (hostname) used by CNI and the dnsname plugin.
// If we are in the pod network namespace use the pod name otherwise the container name
func getCNIPodName(c *Container) string {
	if c.config.NetMode.IsPod() || c.IsInfra() {
		pod, err := c.runtime.state.Pod(c.PodID())
		if err == nil {
			return pod.Name()
		}
	}
	return c.Name()
}

// Tear down a container's network configuration and joins the
// rootless net ns as rootless user
func (r *Runtime) teardownNetwork(ns string, opts types.NetworkOptions) error {
	rootlessNetNS, err := r.GetRootlessNetNs(false)
	if err != nil {
		return err
	}
	tearDownPod := func() error {
		if err := r.network.Teardown(ns, types.TeardownOptions{NetworkOptions: opts}); err != nil {
			return fmt.Errorf("tearing down network namespace configuration for container %s: %w", opts.ContainerID, err)
		}
		return nil
	}

	// rootlessNetNS is nil if we are root
	if rootlessNetNS != nil {
		// execute the cni setup in the rootless net ns
		err = rootlessNetNS.Do(tearDownPod)
		if cerr := rootlessNetNS.Cleanup(r); cerr != nil {
			logrus.WithError(err).Error("failed to clean up rootless netns")
		}
		rootlessNetNS.Lock.Unlock()
	} else {
		err = tearDownPod()
	}
	return err
}

// Tear down a container's CNI network configuration, but do not tear down the
// namespace itself.
func (r *Runtime) teardownCNI(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	logrus.Debugf("Tearing down network namespace at %s for container %s", ctr.state.NetNS.Path(), ctr.ID())

	networks, err := ctr.networks()
	if err != nil {
		return err
	}

	if !ctr.config.NetMode.IsSlirp4netns() && len(networks) > 0 {
		netOpts := ctr.getNetworkOptions(networks)
		return r.teardownNetwork(ctr.state.NetNS.Path(), netOpts)
	}
	return nil
}

// isBridgeNetMode checks if the given network mode is bridge.
// It returns nil when it is set to bridge and an error otherwise.
func isBridgeNetMode(n namespaces.NetworkMode) error {
	if !n.IsBridge() {
		return fmt.Errorf("%q is not supported: %w", n, define.ErrNetworkModeInvalid)
	}
	return nil
}

// Reload only works with containers with a configured network.
// It will tear down, and then reconfigure, the network of the container.
// This is mainly used when a reload of firewall rules wipes out existing
// firewall configuration.
// Efforts will be made to preserve MAC and IP addresses, but this only works if
// the container only joined a single CNI network, and was only assigned a
// single MAC or IP.
// Only works on root containers at present, though in the future we could
// extend this to stop + restart slirp4netns
func (r *Runtime) reloadContainerNetwork(ctr *Container) (map[string]types.StatusBlock, error) {
	if ctr.state.NetNS == nil {
		return nil, fmt.Errorf("container %s network is not configured, refusing to reload: %w", ctr.ID(), define.ErrCtrStateInvalid)
	}
	if err := isBridgeNetMode(ctr.config.NetMode); err != nil {
		return nil, err
	}
	logrus.Infof("Going to reload container %s network", ctr.ID())

	err := r.teardownCNI(ctr)
	if err != nil {
		// teardownCNI will error if the iptables rules do not exists and this is the case after
		// a firewall reload. The purpose of network reload is to recreate the rules if they do
		// not exists so we should not log this specific error as error. This would confuse users otherwise.
		// iptables-legacy and iptables-nft will create different errors make sure to match both.
		b, rerr := regexp.MatchString("Couldn't load target `CNI-[a-f0-9]{24}':No such file or directory|Chain 'CNI-[a-f0-9]{24}' does not exist", err.Error())
		if rerr == nil && !b {
			logrus.Error(err)
		} else {
			logrus.Info(err)
		}
	}

	networkOpts, err := ctr.networks()
	if err != nil {
		return nil, err
	}

	// Set the same network settings as before..
	netStatus := ctr.getNetworkStatus()
	for network, perNetOpts := range networkOpts {
		for name, netInt := range netStatus[network].Interfaces {
			perNetOpts.InterfaceName = name
			perNetOpts.StaticMAC = netInt.MacAddress
			for _, netAddress := range netInt.Subnets {
				perNetOpts.StaticIPs = append(perNetOpts.StaticIPs, netAddress.IPNet.IP)
			}
			// Normally interfaces have a length of 1, only for some special cni configs we could get more.
			// For now just use the first interface to get the ips this should be good enough for most cases.
			break
		}
		networkOpts[network] = perNetOpts
	}
	ctr.perNetworkOpts = networkOpts

	return r.configureNetNS(ctr, ctr.state.NetNS)
}

// Produce an InspectNetworkSettings containing information on the container
// network.
func (c *Container) getContainerNetworkInfo() (*define.InspectNetworkSettings, error) {
	if c.config.NetNsCtr != "" {
		netNsCtr, err := c.runtime.GetContainer(c.config.NetNsCtr)
		if err != nil {
			return nil, err
		}
		// see https://github.com/containers/podman/issues/10090
		// the container has to be locked for syncContainer()
		netNsCtr.lock.Lock()
		defer netNsCtr.lock.Unlock()
		// Have to sync to ensure that state is populated
		if err := netNsCtr.syncContainer(); err != nil {
			return nil, err
		}
		logrus.Debugf("Container %s shares network namespace, retrieving network info of container %s", c.ID(), c.config.NetNsCtr)

		return netNsCtr.getContainerNetworkInfo()
	}

	settings := new(define.InspectNetworkSettings)
	settings.Ports = makeInspectPortBindings(c.config.PortMappings, c.config.ExposedPorts)

	networks, err := c.networks()
	if err != nil {
		return nil, err
	}

	if c.state.NetNS == nil {
		if networkNSPath := c.joinedNetworkNSPath(); networkNSPath != "" {
			if result, err := c.inspectJoinedNetworkNS(networkNSPath); err == nil {
				// fallback to dummy configuration
				settings.InspectBasicNetworkConfig = resultToBasicNetworkConfig(result)
				return settings, nil
			}
			// do not propagate error inspecting a joined network ns
			logrus.Errorf("Inspecting network namespace: %s of container %s: %v", networkNSPath, c.ID(), err)
		}
		// We can't do more if the network is down.

		// We still want to make dummy configurations for each CNI net
		// the container joined.
		if len(networks) > 0 {
			settings.Networks = make(map[string]*define.InspectAdditionalNetwork, len(networks))
			for net, opts := range networks {
				cniNet := new(define.InspectAdditionalNetwork)
				cniNet.NetworkID = net
				cniNet.Aliases = opts.Aliases
				settings.Networks[net] = cniNet
			}
		}

		return settings, nil
	}

	// Set network namespace path
	settings.SandboxKey = c.state.NetNS.Path()

	netStatus := c.getNetworkStatus()
	// If this is empty, we're probably slirp4netns
	if len(netStatus) == 0 {
		return settings, nil
	}

	// If we have networks - handle that here
	if len(networks) > 0 {
		if len(networks) != len(netStatus) {
			return nil, fmt.Errorf("network inspection mismatch: asked to join %d network(s) %v, but have information on %d network(s): %w", len(networks), networks, len(netStatus), define.ErrInternal)
		}

		settings.Networks = make(map[string]*define.InspectAdditionalNetwork)

		for name, opts := range networks {
			result := netStatus[name]
			addedNet := new(define.InspectAdditionalNetwork)
			addedNet.NetworkID = name
			addedNet.Aliases = opts.Aliases
			addedNet.InspectBasicNetworkConfig = resultToBasicNetworkConfig(result)

			settings.Networks[name] = addedNet
		}

		// if not only the default network is connected we can return here
		// otherwise we have to populate the InspectBasicNetworkConfig settings
		_, isDefaultNet := networks[c.runtime.config.Network.DefaultNetwork]
		if !(len(networks) == 1 && isDefaultNet) {
			return settings, nil
		}
	}

	// If not joining networks, we should have at most 1 result
	if len(netStatus) > 1 {
		return nil, fmt.Errorf("should have at most 1 network status result if not joining networks, instead got %d: %w", len(netStatus), define.ErrInternal)
	}

	if len(netStatus) == 1 {
		for _, status := range netStatus {
			settings.InspectBasicNetworkConfig = resultToBasicNetworkConfig(status)
		}
	}
	return settings, nil
}

// resultToBasicNetworkConfig produces an InspectBasicNetworkConfig from a CNI
// result
func resultToBasicNetworkConfig(result types.StatusBlock) define.InspectBasicNetworkConfig {
	config := define.InspectBasicNetworkConfig{}
	interfaceNames := make([]string, 0, len(result.Interfaces))
	for interfaceName := range result.Interfaces {
		interfaceNames = append(interfaceNames, interfaceName)
	}
	// ensure consistent inspect results by sorting
	sort.Strings(interfaceNames)
	for _, interfaceName := range interfaceNames {
		netInt := result.Interfaces[interfaceName]
		for _, netAddress := range netInt.Subnets {
			size, _ := netAddress.IPNet.Mask.Size()
			if netAddress.IPNet.IP.To4() != nil {
				// ipv4
				if config.IPAddress == "" {
					config.IPAddress = netAddress.IPNet.IP.String()
					config.IPPrefixLen = size
					config.Gateway = netAddress.Gateway.String()
				} else {
					config.SecondaryIPAddresses = append(config.SecondaryIPAddresses, define.Address{Addr: netAddress.IPNet.IP.String(), PrefixLength: size})
				}
			} else {
				// ipv6
				if config.GlobalIPv6Address == "" {
					config.GlobalIPv6Address = netAddress.IPNet.IP.String()
					config.GlobalIPv6PrefixLen = size
					config.IPv6Gateway = netAddress.Gateway.String()
				} else {
					config.SecondaryIPv6Addresses = append(config.SecondaryIPv6Addresses, define.Address{Addr: netAddress.IPNet.IP.String(), PrefixLength: size})
				}
			}
		}
		if config.MacAddress == "" {
			config.MacAddress = netInt.MacAddress.String()
		} else {
			config.AdditionalMacAddresses = append(config.AdditionalMacAddresses, netInt.MacAddress.String())
		}
	}
	return config
}
