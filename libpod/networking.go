package libpod

import (
	"net"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/kubelet/network/hostport"
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

// Convert port mapping struct from OCICNI version to one Kubernetes understands
func portMappingToHostport(mappings []ocicni.PortMapping) ([]*hostport.PortMapping, error) {
	newMappings := make([]*hostport.PortMapping, len(mappings))
	for _, port := range mappings {
		var protocol v1.Protocol
		switch strings.ToLower(port.Protocol) {
		case "udp":
			protocol = v1.ProtocolUDP
		case "tcp":
			protocol = v1.ProtocolTCP
		default:
			return nil, errors.Wrapf(ErrInvalidArg, "protocol must be TCP or UDP, instead got %s", port.Protocol)
		}

		newPort := new(hostport.PortMapping)
		newPort.Name = ""
		newPort.HostPort = port.HostPort
		newPort.ContainerPort = port.ContainerPort
		newPort.Protocol = protocol
		newPort.HostIP = port.HostIP

		newMappings = append(newMappings, newPort)
	}
	return newMappings, nil
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

	podNetwork := getPodNetwork(ctr.ID(), ctr.Name(), ctrNS.Path(), ctr.config.PortMappings)

	if err := r.netPlugin.SetUpPod(podNetwork); err != nil {
		return errors.Wrapf(err, "error configuring network namespace for container %s", ctr.ID())
	}

	if len(ctr.config.PortMappings) != 0 {
		ip, err := r.netPlugin.GetPodNetworkStatus(podNetwork)
		if err != nil {
			return errors.Wrapf(err, "failed to get status of network for container %s", ctr.ID())
		}

		ip4 := net.ParseIP(ip).To4()
		if ip4 == nil {
			return errors.Wrapf(err, "failed to parse IPv4 address for container %s", ctr.ID())
		}

		portMappings, err := portMappingToHostport(ctr.config.PortMappings)
		if err != nil {
			return errors.Wrapf(err, "failed to generate port ammpings for container %s", ctr.ID())
		}

		err = r.hostportManager.Add(ctr.ID(), &hostport.PodPortMapping{
			Name:         ctr.Name(),
			PortMappings: portMappings,
			IP:           ip4,
			HostNetwork:  false,
		}, "lo")
		if err != nil {
			return errors.Wrapf(err, "failed to add port mappings for container %s", ctr.ID())
		}
	}

	ctr.state.NetNS = ctrNS

	return nil
}

// Join an existing network namespace
func joinNetNS(path string) (ns.NetNS, error) {
	ns, err := ns.GetNS(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving network namespace at %s", path)
	}

	return ns, nil
}

// Tear down a network namespace
func (r *Runtime) teardownNetNS(ctr *Container) error {
	if ctr.state.NetNS == nil {
		// The container has no network namespace, we're set
		return nil
	}

	portMappings, err := portMappingToHostport(ctr.config.PortMappings)
	if err != nil {
		logrus.Errorf("Failed to generate port mappings for container %s: %v", ctr.ID(), err)
	} else {
		// Only attempt to remove hostport mappings if we successfully
		// converted to hostport-style mappings
		err := r.hostportManager.Remove(ctr.ID(), &hostport.PodPortMapping{
			Name:         ctr.Name(),
			PortMappings: portMappings,
			HostNetwork:  false,
		})
		if err != nil {
			logrus.Errorf("Failed to tear down port mappings for container %s: %v", ctr.ID(), err)
		}
	}

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
