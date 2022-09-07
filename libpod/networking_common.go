//go:build linux || freebsd
// +build linux freebsd

package libpod

import (
	"fmt"
	"regexp"

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
