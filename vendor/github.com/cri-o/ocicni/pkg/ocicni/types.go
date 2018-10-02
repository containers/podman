package ocicni

import (
	"github.com/containernetworking/cni/pkg/types"
)

const (
	// DefaultInterfaceName is the string to be used for the interface name inside the net namespace
	DefaultInterfaceName = "eth0"
	// CNIPluginName is the default name of the plugin
	CNIPluginName = "cni"
)

// PortMapping maps to the standard CNI portmapping Capability
// see: https://github.com/containernetworking/cni/blob/master/CONVENTIONS.md
type PortMapping struct {
	// HostPort is the port number on the host.
	HostPort int32 `json:"hostPort"`
	// ContainerPort is the port number inside the sandbox.
	ContainerPort int32 `json:"containerPort"`
	// Protocol is the protocol of the port mapping.
	Protocol string `json:"protocol"`
	// HostIP is the host ip to use.
	HostIP string `json:"hostIP"`
}

// NetworkConfig is additional configuration for a single CNI network.
type NetworkConfig struct {
	// IP is a static IP to be specified in the network. Can only be used
	// with the hostlocal IP allocator. If left unset, an IP will be
	// dynamically allocated.
	IP string
}

// PodNetwork configures the network of a pod sandbox.
type PodNetwork struct {
	// Name is the name of the sandbox.
	Name string
	// Namespace is the namespace of the sandbox.
	Namespace string
	// ID is the id of the sandbox container.
	ID string
	// NetNS is the network namespace path of the sandbox.
	NetNS string
	// PortMappings is the port mapping of the sandbox.
	PortMappings []PortMapping

	// Networks is a list of CNI network names to attach to the sandbox
	// Leave this list empty to attach the default network to the sandbox
	Networks []string

	// NetworkConfig is configuration specific to a single CNI network.
	// It is optional, and can be omitted for some or all specified networks
	// without issue.
	NetworkConfig map[string]NetworkConfig
}

// CNIPlugin is the interface that needs to be implemented by a plugin
type CNIPlugin interface {
	// Name returns the plugin's name. This will be used when searching
	// for a plugin by name, e.g.
	Name() string

	// SetUpPod is the method called after the sandbox container of
	// the pod has been created but before the other containers of the
	// pod are launched.
	SetUpPod(network PodNetwork) ([]types.Result, error)

	// TearDownPod is the method called before a pod's sandbox container will be deleted
	TearDownPod(network PodNetwork) error

	// Status is the method called to obtain the ipv4 or ipv6 addresses of the pod sandbox
	GetPodNetworkStatus(network PodNetwork) ([]types.Result, error)

	// NetworkStatus returns error if the network plugin is in error state
	Status() error

	// Shutdown terminates all driver operations
	Shutdown() error
}
