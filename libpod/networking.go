package libpod

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Get an OCICNI network config
func getPodNetwork(id, name, nsPath string, ports []ocicni.PortMapping) ocicni.PodNetwork {
	return ocicni.PodNetwork{
		Name:         name,
		Namespace:    name, // TODO is there something else we should put here? We don't know about Kube namespaces
		ID:           id,
		NetNS:        nsPath,
		PortMappings: ports,
	}
}

// Create and configure a new network namespace for a container
func (r *Runtime) configureNetNS(ctr *Container, ctrNS ns.NetNS) (err error) {
	podNetwork := getPodNetwork(ctr.ID(), ctr.Name(), ctrNS.Path(), ctr.config.PortMappings)

	result, err := r.netPlugin.SetUpPod(podNetwork)
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

	logrus.Debugf("Response from CNI plugins: %v", result.String())

	resultStruct, err := cnitypes.GetResult(result)
	if err != nil {
		return errors.Wrapf(err, "error parsing result from CNI plugins")
	}

	ctr.state.NetNS = ctrNS
	ctr.state.IPs = resultStruct.IPs
	ctr.state.Routes = resultStruct.Routes

	// We need to temporarily use iptables to allow the container
	// to resolve DNS until this issue is fixed upstream.
	// https://github.com/containernetworking/plugins/pull/75
	if resultStruct.IPs != nil {
		for _, ip := range resultStruct.IPs {
			iptablesDNS("-I", ip.Address.IP.String())
		}
	}
	return nil
}

// Create and configure a new network namespace for a container
func (r *Runtime) createNetNS(ctr *Container) (err error) {
	ctrNS, err := ns.NewNS()
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

// Get a container's IP address
func (r *Runtime) getContainerIP(ctr *Container) (net.IP, error) {
	if ctr.state.NetNS == nil {
		return nil, errors.Wrapf(ErrInvalidArg, "container %s has no network namespace, cannot get IP", ctr.ID())
	}

	podNetwork := getPodNetwork(ctr.ID(), ctr.Name(), ctr.state.NetNS.Path(), ctr.config.PortMappings)

	ipStr, err := r.netPlugin.GetPodNetworkStatus(podNetwork)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving network status of container %s", ctr.ID())
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, errors.Wrapf(ErrInternal, "error parsing IP address %s for container %s", ipStr, ctr.ID())
	}

	return ip, nil
}

// Tear down a network namespace
func (r *Runtime) teardownNetNS(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	// Because we are using iptables to allow the container to resolve DNS
	// on per IP address, we also need to try to remove the iptables rule
	// on cleanup. Remove when https://github.com/containernetworking/plugins/pull/75
	// is merged.
	for _, ip := range ctr.state.IPs {
		iptablesDNS("-D", ip.Address.IP.String())
	}

	logrus.Debugf("Tearing down network namespace at %s for container %s", ctr.state.NetNS.Path(), ctr.ID())

	podNetwork := getPodNetwork(ctr.ID(), ctr.Name(), ctr.state.NetNS.Path(), ctr.config.PortMappings)

	// The network may have already been torn down, so don't fail here, just log
	if err := r.netPlugin.TearDownPod(podNetwork); err != nil {
		logrus.Errorf("Failed to tear down network namespace for container %s: %v", ctr.ID(), err)
	}

	if err := ctr.state.NetNS.Close(); err != nil {
		return errors.Wrapf(err, "error closing network namespace for container %s", ctr.ID())
	}

	ctr.state.NetNS = nil

	return nil
}
