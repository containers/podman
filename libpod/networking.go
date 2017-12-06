package libpod

import (
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/sirupsen/logrus"
)

// Get an OCICNI network config
func getPodNetwork(id, name, nsPath string, ports []ocicni.PortMappings) ocicni.PodNetwork {
	return ocicni.PodNetwork{
		Name:         name,
		Namespace:    name, // TODO is there something else we should put here? We don't know about Kube namespaces
		ID:           id,
		NetNS:        nsPath,
		PortMappings: ports,
	}
}

// Create and configure a new network namespace
func (r *Runtime) createNetNS(id, name string, ports []ocicni.PortMapping) (n ns.NetNS, err error) {
	ns, err := ns.NewNS()
	if err != nil {
		return nil, errors.Wrapf(err, "error creating network namespace %s", id)
	}
	defer func() {
		if err != nil {
			if err2 := ns.Close(); err2 != nil {
				logrus.Errorf("Error closing partially created network namespace %s: %v", id, err2)
			}
		}
	}()

	podNetwork := getPodNetwork(id, name, ns.Path(), ports)

	if err := r.netPlugin.SetUpPod(podNetwork); err != nil {
		return nil, errors.Wrapf(err, "error configuring network namespace %s", id)
	}

	// TODO hostport mappings for forwarded ports

	return ns, nil
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
func (r *Runtime) teardownNetNS(id, name string, ports []ocicni.PortMapping, ns ns.NetNS) error {
	// TODO hostport mappings for forwarded ports should be undone
	podNetwork := getPodNetwork(id, name, ns.Path(), ports)

	if err := r.netPlugin.TearDownPod(podNetwork); err != nil {
		return errors.Wrapf(err, "failed to remove network namespace %s", id)
	}

	return nil
}
