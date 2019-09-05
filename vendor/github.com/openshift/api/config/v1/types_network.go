package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network holds cluster-wide information about Network.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type Network struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration.
	// +kubebuilder:validation:Required
	// +required
	Spec NetworkSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status NetworkStatus `json:"status"`
}

// NetworkSpec is the desired network configuration.
// As a general rule, this SHOULD NOT be read directly. Instead, you should
// consume the NetworkStatus, as it indicates the currently deployed configuration.
// Currently, changing ClusterNetwork, ServiceNetwork, or NetworkType after
// installation is not supported.
type NetworkSpec struct {
	// IP address pool to use for pod IPs.
	ClusterNetwork []ClusterNetworkEntry `json:"clusterNetwork"`

	// IP address pool for services.
	// Currently, we only support a single entry here.
	ServiceNetwork []string `json:"serviceNetwork"`

	// NetworkType is the plugin that is to be deployed (e.g. OpenShiftSDN).
	// This should match a value that the cluster-network-operator understands,
	// or else no networking will be installed.
	// Currently supported values are:
	// - OpenShiftSDN
	NetworkType string `json:"networkType"`

	// externalIP defines configuration for controllers that
	// affect Service.ExternalIP
	// +optional
	ExternalIP *ExternalIPConfig `json:"externalIP,omitempty"`
}

// NetworkStatus is the current network configuration.
type NetworkStatus struct {
	// IP address pool to use for pod IPs.
	ClusterNetwork []ClusterNetworkEntry `json:"clusterNetwork,omitempty"`

	// IP address pool for services.
	// Currently, we only support a single entry here.
	ServiceNetwork []string `json:"serviceNetwork,omitempty"`

	// NetworkType is the plugin that is deployed (e.g. OpenShiftSDN).
	NetworkType string `json:"networkType,omitempty"`

	// ClusterNetworkMTU is the MTU for inter-pod networking.
	ClusterNetworkMTU int `json:"clusterNetworkMTU,omitempty"`
}

// ClusterNetworkEntry is a contiguous block of IP addresses from which pod IPs
// are allocated.
type ClusterNetworkEntry struct {
	// The complete block for pod IPs.
	CIDR string `json:"cidr"`

	// The size (prefix) of block to allocate to each node.
	HostPrefix uint32 `json:"hostPrefix"`
}

// ExternalIPConfig specifies some IP blocks relevant for the ExternalIP field
// of a Service resource.
type ExternalIPConfig struct {
	// policy is a set of restrictions applied to the ExternalIP field.
	// If nil, any value is allowed for an ExternalIP. If the empty/zero
	// policy is supplied, then ExternalIP is not allowed to be set.
	// +optional
	Policy *ExternalIPPolicy `json:"policy,omitempty"`

	// autoAssignCIDRs is a list of CIDRs from which to automatically assign
	// Service.ExternalIP. These are assigned when the service is of type
	// LoadBalancer. In general, this is only useful for bare-metal clusters.
	// In Openshift 3.x, this was misleadingly called "IngressIPs".
	// Automatically assigned External IPs are not affected by any
	// ExternalIPPolicy rules.
	// Currently, only one entry may be provided.
	// +optional
	AutoAssignCIDRs []string `json:"autoAssignCIDRs,omitempty"`
}

// ExternalIPPolicy configures exactly which IPs are allowed for the ExternalIP
// field in a Service. If the zero struct is supplied, then none are permitted.
// The policy controller always allows automatically assigned external IPs.
type ExternalIPPolicy struct {
	// allowedCIDRs is the list of allowed CIDRs.
	AllowedCIDRs []string `json:"allowedCIDRs,omitempty"`

	// rejectedCIDRs is the list of disallowed CIDRs. These take precedence
	// over allowedCIDRs.
	// +optional
	RejectedCIDRs []string `json:"rejectedCIDRs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []Network `json:"items"`
}
