package server

import (
	"fmt"
	"net"

	"github.com/kubernetes-incubator/cri-o/libkpod/sandbox"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/pkg/kubelet/network/hostport"
)

// networkStart sets up the sandbox's network and returns the pod IP on success
// or an error
func (s *Server) networkStart(hostNetwork bool, sb *sandbox.Sandbox) (string, error) {
	if hostNetwork {
		return s.BindAddress(), nil
	}

	podNetwork := newPodNetwork(sb)
	err := s.netPlugin.SetUpPod(podNetwork)
	if err != nil {
		return "", fmt.Errorf("failed to create pod network sandbox %s(%s): %v", sb.Name(), sb.ID(), err)
	}

	var ip string
	if ip, err = s.netPlugin.GetPodNetworkStatus(podNetwork); err != nil {
		return "", fmt.Errorf("failed to get network status for pod sandbox %s(%s): %v", sb.Name(), sb.ID(), err)
	}

	if len(sb.PortMappings()) > 0 {
		ip4 := net.ParseIP(ip).To4()
		if ip4 == nil {
			return "", fmt.Errorf("failed to get valid ipv4 address for sandbox %s(%s)", sb.Name(), sb.ID())
		}

		if err = s.hostportManager.Add(sb.ID(), &hostport.PodPortMapping{
			Name:         sb.Name(),
			PortMappings: sb.PortMappings(),
			IP:           ip4,
			HostNetwork:  false,
		}, "lo"); err != nil {
			return "", fmt.Errorf("failed to add hostport mapping for sandbox %s(%s): %v", sb.Name(), sb.ID(), err)
		}

	}
	return ip, nil
}

// networkStop cleans up and removes a pod's network.  It is best-effort and
// must call the network plugin even if the network namespace is already gone
func (s *Server) networkStop(hostNetwork bool, sb *sandbox.Sandbox) error {
	if !hostNetwork {
		if err := s.hostportManager.Remove(sb.ID(), &hostport.PodPortMapping{
			Name:         sb.Name(),
			PortMappings: sb.PortMappings(),
			HostNetwork:  false,
		}); err != nil {
			logrus.Warnf("failed to remove hostport for pod sandbox %s(%s): %v",
				sb.Name(), sb.ID(), err)
		}

		podNetwork := newPodNetwork(sb)
		if err := s.netPlugin.TearDownPod(podNetwork); err != nil {
			logrus.Warnf("failed to destroy network for pod sandbox %s(%s): %v",
				sb.Name(), sb.ID(), err)
		}
	}

	return nil
}
