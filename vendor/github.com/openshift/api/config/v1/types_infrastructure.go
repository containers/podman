package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Infrastructure holds cluster-wide information about Infrastructure.  The canonical name is `cluster`
type Infrastructure struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +kubebuilder:validation:Required
	// +required
	Spec InfrastructureSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status InfrastructureStatus `json:"status"`
}

// InfrastructureSpec contains settings that apply to the cluster infrastructure.
type InfrastructureSpec struct {
	// cloudConfig is a reference to a ConfigMap containing the cloud provider configuration file.
	// This configuration file is used to configure the Kubernetes cloud provider integration
	// when using the built-in cloud provider integration or the external cloud controller manager.
	// The namespace for this config map is openshift-config.
	// +optional
	CloudConfig ConfigMapFileReference `json:"cloudConfig"`
}

// InfrastructureStatus describes the infrastructure the cluster is leveraging.
type InfrastructureStatus struct {
	// infrastructureName uniquely identifies a cluster with a human friendly name.
	// Once set it should not be changed. Must be of max length 27 and must have only
	// alphanumeric or hyphen characters.
	InfrastructureName string `json:"infrastructureName"`

	// platform is the underlying infrastructure provider for the cluster.
	//
	// Deprecated: Use platformStatus.type instead.
	Platform PlatformType `json:"platform,omitempty"`

	// platformStatus holds status information specific to the underlying
	// infrastructure provider.
	// +optional
	PlatformStatus *PlatformStatus `json:"platformStatus,omitempty"`

	// etcdDiscoveryDomain is the domain used to fetch the SRV records for discovering
	// etcd servers and clients.
	// For more info: https://github.com/etcd-io/etcd/blob/329be66e8b3f9e2e6af83c123ff89297e49ebd15/Documentation/op-guide/clustering.md#dns-discovery
	EtcdDiscoveryDomain string `json:"etcdDiscoveryDomain"`

	// apiServerURL is a valid URI with scheme(http/https), address and
	// port.  apiServerURL can be used by components like the web console
	// to tell users where to find the Kubernetes API.
	APIServerURL string `json:"apiServerURL"`

	// apiServerInternalURL is a valid URI with scheme(http/https),
	// address and port.  apiServerInternalURL can be used by components
	// like kubelets, to contact the Kubernetes API server using the
	// infrastructure provider rather than Kubernetes networking.
	APIServerInternalURL string `json:"apiServerInternalURI"`
}

// PlatformType is a specific supported infrastructure provider.
type PlatformType string

const (
	// AWSPlatformType represents Amazon Web Services infrastructure.
	AWSPlatformType PlatformType = "AWS"

	// AzurePlatformType represents Microsoft Azure infrastructure.
	AzurePlatformType PlatformType = "Azure"

	// BareMetalPlatformType represents managed bare metal infrastructure.
	BareMetalPlatformType PlatformType = "BareMetal"

	// GCPPlatformType represents Google Cloud Platform infrastructure.
	GCPPlatformType PlatformType = "GCP"

	// LibvirtPlatformType represents libvirt infrastructure.
	LibvirtPlatformType PlatformType = "Libvirt"

	// OpenStackPlatformType represents OpenStack infrastructure.
	OpenStackPlatformType PlatformType = "OpenStack"

	// NonePlatformType means there is no infrastructure provider.
	NonePlatformType PlatformType = "None"

	// VSpherePlatformType represents VMWare vSphere infrastructure.
	VSpherePlatformType PlatformType = "VSphere"

	// OvirtPlatformType represents oVirt/RHV infrastructure.
	OvirtPlatformType PlatformType = "oVirt"
)

// PlatformStatus holds the current status specific to the underlying infrastructure provider
// of the current cluster. Since these are used at status-level for the underlying cluster, it
// is supposed that only one of the status structs is set.
type PlatformStatus struct {
	// type is the underlying infrastructure provider for the cluster. This
	// value controls whether infrastructure automation such as service load
	// balancers, dynamic volume provisioning, machine creation and deletion, and
	// other integrations are enabled. If None, no infrastructure automation is
	// enabled. Allowed values are "AWS", "Azure", "BareMetal", "GCP", "Libvirt",
	// "OpenStack", "VSphere", "oVirt", and "None". Individual components may not support
	// all platforms, and must handle unrecognized platforms as None if they do
	// not support that platform.
	Type PlatformType `json:"type"`

	// AWS contains settings specific to the Amazon Web Services infrastructure provider.
	// +optional
	AWS *AWSPlatformStatus `json:"aws,omitempty"`

	// Azure contains settings specific to the Azure infrastructure provider.
	// +optional
	Azure *AzurePlatformStatus `json:"azure,omitempty"`

	// GCP contains settings specific to the Google Cloud Platform infrastructure provider.
	// +optional
	GCP *GCPPlatformStatus `json:"gcp,omitempty"`

	// BareMetal contains settings specific to the BareMetal platform.
	// +optional
	BareMetal *BareMetalPlatformStatus `json:"baremetal,omitempty"`

	// OpenStack contains settings specific to the OpenStack infrastructure provider.
	// +optional
	OpenStack *OpenStackPlatformStatus `json:"openstack,omitempty"`
}

// AWSPlatformStatus holds the current status of the Amazon Web Services infrastructure provider.
type AWSPlatformStatus struct {
	// region holds the default AWS region for new AWS resources created by the cluster.
	Region string `json:"region"`
}

// AzurePlatformStatus holds the current status of the Azure infrastructure provider.
type AzurePlatformStatus struct {
	// resourceGroupName is the Resource Group for new Azure resources created for the cluster.
	ResourceGroupName string `json:"resourceGroupName"`
}

// GCPPlatformStatus holds the current status of the Google Cloud Platform infrastructure provider.
type GCPPlatformStatus struct {
	// resourceGroupName is the Project ID for new GCP resources created for the cluster.
	ProjectID string `json:"projectID"`

	// region holds the region for new GCP resources created for the cluster.
	Region string `json:"region"`
}

// BareMetalPlatformStatus holds the current status of the BareMetal infrastructure provider.
type BareMetalPlatformStatus struct {
	// apiServerInternalIP is an IP address to contact the Kubernetes API server that can be used
	// by components inside the cluster, like kubelets using the infrastructure rather
	// than Kubernetes networking. It is the IP that the Infrastructure.status.apiServerInternalURI
	// points to. It is the IP for a self-hosted load balancer in front of the API servers.
	APIServerInternalIP string `json:"apiServerInternalIP,omitempty"`

	// ingressIP is an external IP which routes to the default ingress controller.
	// The IP is a suitable target of a wildcard DNS record used to resolve default route host names.
	IngressIP string `json:"ingressIP,omitempty"`

	// nodeDNSIP is the IP address for the internal DNS used by the
	// nodes. Unlike the one managed by the DNS operator, `NodeDNSIP`
	// provides name resolution for the nodes themselves. There is no DNS-as-a-service for
	// BareMetal deployments. In order to minimize necessary changes to the
	// datacenter DNS, a DNS service is hosted as a static pod to serve those hostnames
	// to the nodes in the cluster.
	NodeDNSIP string `json:"nodeDNSIP,omitempty"`
}

// OpenStackPlatformStatus holds the current status of the OpenStack infrastructure provider.
type OpenStackPlatformStatus struct {
	// apiServerInternalIP is an IP address to contact the Kubernetes API server that can be used
	// by components inside the cluster, like kubelets using the infrastructure rather
	// than Kubernetes networking. It is the IP that the Infrastructure.status.apiServerInternalURI
	// points to. It is the IP for a self-hosted load balancer in front of the API servers.
	APIServerInternalIP string `json:"apiServerInternalIP,omitempty"`

	// cloudName is the name of the desired OpenStack cloud in the
	// client configuration file (`clouds.yaml`).
	CloudName string `json:"cloudName,omitempty"`

	// ingressIP is an external IP which routes to the default ingress controller.
	// The IP is a suitable target of a wildcard DNS record used to resolve default route host names.
	IngressIP string `json:"ingressIP,omitempty"`

	// nodeDNSIP is the IP address for the internal DNS used by the
	// nodes. Unlike the one managed by the DNS operator, `NodeDNSIP`
	// provides name resolution for the nodes themselves. There is no DNS-as-a-service for
	// OpenStack deployments. In order to minimize necessary changes to the
	// datacenter DNS, a DNS service is hosted as a static pod to serve those hostnames
	// to the nodes in the cluster.
	NodeDNSIP string `json:"nodeDNSIP,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureList is
type InfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []Infrastructure `json:"items"`
}
