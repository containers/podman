// +build linux

package libpod

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/containers/libpod/pkg/netns"
	"github.com/containers/libpod/utils"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Get an OCICNI network config
func getPodNetwork(id, name, nsPath string, networks []string, ports []ocicni.PortMapping) ocicni.PodNetwork {
	return ocicni.PodNetwork{
		Name:         name,
		Namespace:    name, // TODO is there something else we should put here? We don't know about Kube namespaces
		ID:           id,
		NetNS:        nsPath,
		PortMappings: ports,
		Networks:     networks,
	}
}

// Create and configure a new network namespace for a container
func (r *Runtime) configureNetNS(ctr *Container, ctrNS ns.NetNS) (err error) {
	podNetwork := getPodNetwork(ctr.ID(), ctr.Name(), ctrNS.Path(), ctr.config.Networks, ctr.config.PortMappings)

	results, err := r.netPlugin.SetUpPod(podNetwork)
	if err != nil {
		return errors.Wrapf(err, "error configuring network namespace for container %s", ctr.ID())
	}
	defer func() {
		if err != nil {
			if err2 := r.netPlugin.TearDownPod(podNetwork); err2 != nil {
				logrus.Errorf("Error tearing down partially created network namespace for container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	ctr.state.NetNS = ctrNS
	ctr.state.NetworkStatus = make([]*cnitypes.Result, 0)
	for idx, r := range results {
		logrus.Debugf("[%d] CNI result: %v", idx, r.String())
		resultCurrent, err := cnitypes.GetResult(r)
		if err != nil {
			return errors.Wrapf(err, "error parsing CNI plugin result %q: %v", r.String(), err)
		}
		ctr.state.NetworkStatus = append(ctr.state.NetworkStatus, resultCurrent)
	}

	for _, r := range ctr.state.NetworkStatus {
		// We need to temporarily use iptables to allow the container
		// to resolve DNS until this issue is fixed upstream.
		// https://github.com/containernetworking/plugins/pull/75
		for _, ip := range r.IPs {
			if ip.Address.IP.To4() != nil {
				iptablesDNS("-I", ip.Address.IP.String())
			}
		}
	}

	return nil
}

// Create and configure a new network namespace for a container
func (r *Runtime) createNetNS(ctr *Container) (err error) {
	ctrNS, err := netns.NewNS()
	if err != nil {
		return errors.Wrapf(err, "error creating network namespace for container %s", ctr.ID())
	}
	defer func() {
		if err != nil {
			if err2 := ctrNS.Close(); err2 != nil {
				logrus.Errorf("Error closing partially created network namespace for container %s: %v", ctr.ID(), err2)
			}
		}
	}()

	logrus.Debugf("Made network namespace at %s for container %s", ctrNS.Path(), ctr.ID())
	return r.configureNetNS(ctr, ctrNS)
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
		return errors.Wrapf(err, "failed to start process")
	}

	b := make([]byte, 16)
	if _, err := syncR.Read(b); err != nil {
		return errors.Wrapf(err, "failed to read from sync pipe")
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
	return r.configureNetNS(ctr, netNS)
}

// iptablesDNS accepts an arg (-I|-D) and IP address of the container and then
// generates an iptables command to either add or subtract the needed rule
func iptablesDNS(arg, ip string) error {
	iptablesCmd := []string{"-t", "filter", arg, "FORWARD", "-s", ip, "!", "-o", ip, "-j", "ACCEPT"}
	logrus.Debug("Running iptables command: ", strings.Join(iptablesCmd, " "))
	_, err := utils.ExecCmd("iptables", iptablesCmd...)
	if err != nil {
		logrus.Error(err)
	}
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

	// Because we are using iptables to allow the container to resolve DNS
	// on per IP address, we also need to try to remove the iptables rule
	// on cleanup. Remove when https://github.com/containernetworking/plugins/pull/75
	// is merged.
	for _, r := range ctr.state.NetworkStatus {
		for _, ip := range r.IPs {
			if ip.Address.IP.To4() != nil {
				iptablesDNS("-D", ip.Address.IP.String())
			}
		}
	}

	logrus.Debugf("Tearing down network namespace at %s for container %s", ctr.state.NetNS.Path(), ctr.ID())

	podNetwork := getPodNetwork(ctr.ID(), ctr.Name(), ctr.state.NetNS.Path(), ctr.config.Networks, ctr.config.PortMappings)

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

func getContainerNetIO(ctr *Container) (*netlink.LinkStatistics, error) {
	var netStats *netlink.LinkStatistics
	err := ns.WithNetNSPath(ctr.state.NetNS.Path(), func(_ ns.NetNS) error {
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
