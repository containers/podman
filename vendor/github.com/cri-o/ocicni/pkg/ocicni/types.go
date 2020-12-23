package ocicni

import (
	"context"

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

// IpRange maps to the standard CNI ipRanges Capability
// see: https://github.com/containernetworking/cni/blob/master/CONVENTIONS.md
type IpRange struct {
	// Subnet is the whole CIDR
	Subnet string `json:"subnet"`
	// RangeStart is the first available IP in subnet
	RangeStart string `json:"rangeStart,omitempty"`
	// RangeEnd is the last available IP in subnet
	RangeEnd string `json:"rangeEnd,omitempty"`
	// Gateway is the gateway of subnet
	Gateway string `json:"gateway,omitempty"`
}

// RuntimeConfig is additional configuration for a single CNI network that
// is pod-specific rather than general to the network.
type RuntimeConfig struct {
	// IP is a static IP to be specified in the network. Can only be used
	// with the hostlocal IP allocator. If left unset, an IP will be
	// dynamically allocated.
	IP string
	// MAC is a static MAC address to be assigned to the network interface.
	// If left unset, a MAC will be dynamically allocated.
	MAC string
	// PortMappings is the port mapping of the sandbox.
	PortMappings []PortMapping
	// Bandwidth is the bandwidth limiting of the pod
	Bandwidth *BandwidthConfig
	// IpRanges is the ip range gather which is used for address allocation
	IpRanges [][]IpRange
}

// BandwidthConfig maps to the standard CNI bandwidth Capability
// see: https://github.com/containernetworking/cni/blob/master/CONVENTIONS.md
type BandwidthConfig struct {
	// IngressRate is a limit for incoming traffic in bps
	IngressRate  uint64
	IngressBurst uint64

	// EgressRate is a limit for outgoing traffic in bps
	EgressRate  uint64
	EgressBurst uint64
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

	// Networks is a list of CNI network names (and optional interface
	// names) to attach to the sandbox. Leave this list empty to attach the
	// default network to the sandbox
	Networks []NetAttachment

	// NetworkConfig is configuration specific to a single CNI network.
	// It is optional, and can be omitted for some or all specified networks
	// without issue.
	RuntimeConfig map[string]RuntimeConfig

	// Aliases are network-scoped names for resolving a container
	// by name. The key value is the network name and the value is
	// is a string slice of aliases
	Aliases map[string][]string
}

// NetAttachment describes a container network attachment
type NetAttachment struct {
	// NetName contains the name of the CNI network to which the container
	// should be or is attached
	Name string
	// Ifname contains the optional interface name of the attachment
	Ifname string
}

// NetResult contains the result the network attachment operation
type NetResult struct {
	// Result is the CNI Result
	Result types.Result
	// NetAttachment contains the network and interface names of this
	// network attachment
	NetAttachment
}

// CNIPlugin is the interface that needs to be implemented by a plugin
type CNIPlugin interface {
	// Name returns the plugin's name. This will be used when searching
	// for a plugin by name, e.g.
	Name() string

	// GetDefaultNetworkName returns the name of the plugin's default
	// network.
	GetDefaultNetworkName() string

	// SetUpPod is the method called after the sandbox container of
	// the pod has been created but before the other containers of the
	// pod are launched.
	SetUpPod(network PodNetwork) ([]NetResult, error)

	// SetUpPodWithContext is the same as SetUpPod but takes a context
	SetUpPodWithContext(ctx context.Context, network PodNetwork) ([]NetResult, error)

	// TearDownPod is the method called before a pod's sandbox container will be deleted
	TearDownPod(network PodNetwork) error

	// TearDownPodWithContext is the same as TearDownPod but takes a context
	TearDownPodWithContext(ctx context.Context, network PodNetwork) error

	// GetPodNetworkStatus is the method called to obtain the ipv4 or ipv6 addresses of the pod sandbox
	GetPodNetworkStatus(network PodNetwork) ([]NetResult, error)

	// GetPodNetworkStatusWithContext is the same as GetPodNetworkStatus but takes a context
	GetPodNetworkStatusWithContext(ctx context.Context, network PodNetwork) ([]NetResult, error)

	// NetworkStatus returns error if the network plugin is in error state
	Status() error

	// Shutdown terminates all driver operations
	Shutdown() error
}
