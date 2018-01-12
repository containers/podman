package ocicni

import (
	"github.com/containernetworking/cni/pkg/types"
)

const (
	// DefaultInterfaceName is the string to be used for the interface name inside the net namespace
	DefaultInterfaceName = "eth0"
	// CNIPluginName is the default name of the plugin
	CNIPluginName = "cni"
	// DefaultNetDir is the place to look for CNI Network
	DefaultNetDir = "/etc/cni/net.d"
	// DefaultCNIDir is the place to look for cni config files
	DefaultCNIDir = "/opt/cni/bin"
	// VendorCNIDirTemplate is the template for looking up vendor specific cni config/executable files
	VendorCNIDirTemplate = "%s/opt/%s/bin"
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
}

// CNIPlugin is the interface that needs to be implemented by a plugin
type CNIPlugin interface {
	// Name returns the plugin's name. This will be used when searching
	// for a plugin by name, e.g.
	Name() string

	// SetUpPod is the method called after the sandbox container of
	// the pod has been created but before the other containers of the
	// pod are launched.
	SetUpPod(network PodNetwork) (types.Result, error)

	// TearDownPod is the method called before a pod's sandbox container will be deleted
	TearDownPod(network PodNetwork) error

	// Status is the method called to obtain the ipv4 or ipv6 addresses of the pod sandbox
	GetPodNetworkStatus(network PodNetwork) (string, error)

	// NetworkStatus returns error if the network plugin is in error state
	Status() error
}
