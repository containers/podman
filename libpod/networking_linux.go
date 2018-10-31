// +build linux

package libpod

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/libpod/pkg/firewall"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/containers/libpod/pkg/netns"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Get an OCICNI network config
func (r *Runtime) getPodNetwork(id, name, nsPath string, networks []string, ports []ocicni.PortMapping, staticIP net.IP) ocicni.PodNetwork {
	network := ocicni.PodNetwork{
		Name:         name,
		Namespace:    name, // TODO is there something else we should put here? We don't know about Kube namespaces
		ID:           id,
		NetNS:        nsPath,
		PortMappings: ports,
		Networks:     networks,
	}

	if staticIP != nil {
		defaultNetwork := r.netPlugin.GetDefaultNetworkName()

		network.Networks = []string{defaultNetwork}
		network.NetworkConfig = make(map[string]ocicni.NetworkConfig)
		network.NetworkConfig[defaultNetwork] = ocicni.NetworkConfig{IP: staticIP.String()}
	}

	return network
}

// Create and configure a new network namespace for a container
func (r *Runtime) configureNetNS(ctr *Container, ctrNS ns.NetNS) ([]*cnitypes.Result, error) {
	podNetwork := r.getPodNetwork(ctr.ID(), ctr.Name(), ctrNS.Path(), ctr.config.Networks, ctr.config.PortMappings, ctr.config.StaticIP)

	results, err := r.netPlugin.SetUpPod(podNetwork)
	if err != nil {
		return nil, errors.Wrapf(err, "error configuring network namespace for container %s", ctr.ID())
	}
	defer func() {
		if err != nil {
			if err2 := r.netPlugin.TearDownPod(podNetwork); err2 != nil {
				logrus.Errorf("Error tearing down partially created network namespace for container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	networkStatus := make([]*cnitypes.Result, 1)
	for idx, r := range results {
		logrus.Debugf("[%d] CNI result: %v", idx, r.String())
		resultCurrent, err := cnitypes.GetResult(r)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing CNI plugin result %q: %v", r.String(), err)
		}
		networkStatus = append(ctr.state.NetworkStatus, resultCurrent)
	}

	// Add firewall rules to ensure the container has network access.
	// Will not be necessary once CNI firewall plugin merges upstream.
	// https://github.com/containernetworking/plugins/pull/75
	for _, netStatus := range ctr.state.NetworkStatus {
		firewallConf := &firewall.FirewallNetConf{
			PrevResult: netStatus,
		}
		if err := r.firewallBackend.Add(firewallConf); err != nil {
			return nil, errors.Wrapf(err, "error adding firewall rules for container %s", ctr.ID())
		}
	}

	return networkStatus, nil
}

// Create and configure a new network namespace for a container
func (r *Runtime) createNetNS(ctr *Container) (ns.NetNS, []*cnitypes.Result, error) {
	ctrNS, err := netns.NewNS()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error creating network namespace for container %s", ctr.ID())
	}
	defer func() {
		if err != nil {
			if err2 := ctrNS.Close(); err2 != nil {
				logrus.Errorf("Error closing partially created network namespace for container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	logrus.Debugf("Made network namespace at %s for container %s", ctrNS.Path(), ctr.ID())

	networkStatus, err := r.configureNetNS(ctr, ctrNS)
	return ctrNS, networkStatus, err
}

// Configure the network namespace for a rootless container
func (r *Runtime) setupRootlessNetNS(ctr *Container) (err error) {
	defer ctr.rootlessSlirpSyncR.Close()
	defer ctr.rootlessSlirpSyncW.Close()

	path, err := exec.LookPath("slirp4netns")
	if err != nil {
		logrus.Errorf("could not find slirp4netns, the network namespace won't be configured: %v", err)
		return nil
	}

	syncR, syncW, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "failed to open pipe")
	}
	defer syncR.Close()
	defer syncW.Close()

	cmd := exec.Command(path, "-c", "-e", "3", "-r", "4", fmt.Sprintf("%d", ctr.state.PID), "tap0")

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, ctr.rootlessSlirpSyncR, syncW)

	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start slirp4netns process")
	}
	defer cmd.Process.Release()

	b := make([]byte, 16)
	for {
		if err := syncR.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return errors.Wrapf(err, "error setting slirp4netns pipe timeout")
		}
		if _, err := syncR.Read(b); err == nil {
			break
		} else {
			if os.IsTimeout(err) {
				// Check if the process is still running.
				var status syscall.WaitStatus
				_, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil)
				if err != nil {
					return errors.Wrapf(err, "failed to read slirp4netns process status")
				}
				if status.Exited() || status.Signaled() {
					return errors.New("slirp4netns failed")
				}

				continue
			}
			return errors.Wrapf(err, "failed to read from slirp4netns sync pipe")
		}
	}
	return nil
}

// Configure the network namespace using the container process
func (r *Runtime) setupNetNS(ctr *Container) (err error) {
	nsProcess := fmt.Sprintf("/proc/%d/ns/net", ctr.state.PID)

	b := make([]byte, 16)

	if _, err := rand.Reader.Read(b); err != nil {
		return errors.Wrapf(err, "failed to generate random netns name")
	}

	nsPath := fmt.Sprintf("/var/run/netns/cni-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	if err := os.MkdirAll(filepath.Dir(nsPath), 0711); err != nil {
		return errors.Wrapf(err, "cannot create %s", filepath.Dir(nsPath))
	}

	mountPointFd, err := os.Create(nsPath)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", nsPath)
	}
	mountPointFd.Close()

	if err := unix.Mount(nsProcess, nsPath, "none", unix.MS_BIND, ""); err != nil {
		return errors.Wrapf(err, "cannot mount %s", nsPath)
	}

	netNS, err := ns.GetNS(nsPath)
	if err != nil {
		return err
	}
	networkStatus, err := r.configureNetNS(ctr, netNS)

	// Assign NetNS attributes to container
	ctr.state.NetNS = netNS
	ctr.state.NetworkStatus = networkStatus
	return err
}

// Join an existing network namespace
func joinNetNS(path string) (ns.NetNS, error) {
	ns, err := ns.GetNS(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving network namespace at %s", path)
	}

	return ns, nil
}

// Close a network namespace.
// Differs from teardownNetNS() in that it will not attempt to undo the setup of
// the namespace, but will instead only close the open file descriptor
func (r *Runtime) closeNetNS(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	if err := ctr.state.NetNS.Close(); err != nil {
		return errors.Wrapf(err, "error closing network namespace for container %s", ctr.ID())
	}

	ctr.state.NetNS = nil

	return nil
}

// Tear down a network namespace, undoing all state associated with it.
// The CNI firewall rules will be removed, the namespace will be unmounted,
// and the file descriptor associated with it closed.
func (r *Runtime) teardownNetNS(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	// Remove firewall rules we added on configuring the container.
	// Will not be necessary once CNI firewall plugin merges upstream.
	// https://github.com/containernetworking/plugins/pull/75
	for _, netStatus := range ctr.state.NetworkStatus {
		firewallConf := &firewall.FirewallNetConf{
			PrevResult: netStatus,
		}
		if err := r.firewallBackend.Del(firewallConf); err != nil {
			return errors.Wrapf(err, "error removing firewall rules for container %s", ctr.ID())
		}
	}

	logrus.Debugf("Tearing down network namespace at %s for container %s", ctr.state.NetNS.Path(), ctr.ID())

	podNetwork := r.getPodNetwork(ctr.ID(), ctr.Name(), ctr.state.NetNS.Path(), ctr.config.Networks, ctr.config.PortMappings, ctr.config.StaticIP)

	// The network may have already been torn down, so don't fail here, just log
	if err := r.netPlugin.TearDownPod(podNetwork); err != nil {
		return errors.Wrapf(err, "error tearing down CNI namespace configuration for container %s", ctr.ID())
	}

	// First unmount the namespace
	if err := netns.UnmountNS(ctr.state.NetNS); err != nil {
		return errors.Wrapf(err, "error unmounting network namespace for container %s", ctr.ID())
	}

	// Now close the open file descriptor
	if err := ctr.state.NetNS.Close(); err != nil {
		return errors.Wrapf(err, "error closing network namespace for container %s", ctr.ID())
	}

	ctr.state.NetNS = nil

	return nil
}

func getContainerNetNS(ctr *Container) (string, error) {
	if ctr.state.NetNS != nil {
		return ctr.state.NetNS.Path(), nil
	}
	if ctr.config.NetNsCtr != "" {
		c, err := ctr.runtime.GetContainer(ctr.config.NetNsCtr)
		if err != nil {
			return "", err
		}
		if err = c.syncContainer(); err != nil {
			return "", err
		}
		return c.state.NetNS.Path(), nil
	}
	return "", nil
}

func getContainerNetIO(ctr *Container) (*netlink.LinkStatistics, error) {
	var netStats *netlink.LinkStatistics
	netNSPath, netPathErr := getContainerNetNS(ctr)
	if netPathErr != nil {
		return nil, netPathErr
	}
	if netNSPath == "" {
		// If netNSPath is empty, it was set as none, and no netNS was set up
		// this is a valid state and thus return no error, nor any statistics
		return nil, nil
	}
	err := ns.WithNetNSPath(netNSPath, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ocicni.DefaultInterfaceName)
		if err != nil {
			return err
		}
		netStats = link.Attrs().Statistics
		return nil
	})
	return netStats, err
}

func (c *Container) getContainerNetworkInfo(data *inspect.ContainerInspectData) *inspect.ContainerInspectData {
	if c.state.NetNS != nil && len(c.state.NetworkStatus) > 0 {
		// Report network settings from the first pod network
		result := c.state.NetworkStatus[0]
		// Go through our IP addresses
		for _, ctrIP := range result.IPs {
			ipWithMask := ctrIP.Address.String()
			splitIP := strings.Split(ipWithMask, "/")
			mask, _ := strconv.Atoi(splitIP[1])
			if ctrIP.Version == "4" {
				data.NetworkSettings.IPAddress = splitIP[0]
				data.NetworkSettings.IPPrefixLen = mask
				data.NetworkSettings.Gateway = ctrIP.Gateway.String()
			} else {
				data.NetworkSettings.GlobalIPv6Address = splitIP[0]
				data.NetworkSettings.GlobalIPv6PrefixLen = mask
				data.NetworkSettings.IPv6Gateway = ctrIP.Gateway.String()
			}
		}

		// Set network namespace path
		data.NetworkSettings.SandboxKey = c.state.NetNS.Path()

		// Set MAC address of interface linked with network namespace path
		for _, i := range result.Interfaces {
			if i.Sandbox == data.NetworkSettings.SandboxKey {
				data.NetworkSettings.MacAddress = i.Mac
			}
		}
	}
	return data
}
