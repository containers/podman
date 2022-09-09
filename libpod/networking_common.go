//go:build linux || freebsd
// +build linux freebsd

package libpod

import (
	"fmt"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/machine"
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
