package specgen

import (
	"net"
)

// PodBasicConfig contains basic configuration options for pods.
type PodBasicConfig struct {
	// Name is the name of the pod.
	// If not provided, a name will be generated when the pod is created.
	// Optional.
	Name string `json:"name,omitempty"`
	// Hostname is the pod's hostname. If not set, the name of the pod will
	// be used (if a name was not provided here, the name auto-generated for
	// the pod will be used). This will be used by the infra container and
	// all containers in the pod as long as the UTS namespace is shared.
	// Optional.
	Hostname string `json:"hostname,omitempty"`
	// Labels are key-value pairs that are used to add metadata to pods.
	// Optional.
	Labels map[string]string `json:"labels,omitempty"`
	// NoInfra tells the pod not to create an infra container. If this is
	// done, many networking-related options will become unavailable.
	// Conflicts with setting any options in PodNetworkConfig, and the
	// InfraCommand and InfraImages in this struct.
	// Optional.
	NoInfra bool `json:"no_infra,omitempty"`
	// InfraConmonPidFile is a custom path to store the infra container's
	// conmon PID.
	InfraConmonPidFile string `json:"infra_conmon_pid_file,omitempty"`
	// InfraCommand sets the command that will be used to start the infra
	// container.
	// If not set, the default set in the Libpod configuration file will be
	// used.
	// Conflicts with NoInfra=true.
	// Optional.
	InfraCommand []string `json:"infra_command,omitempty"`
	// InfraImage is the image that will be used for the infra container.
	// If not set, the default set in the Libpod configuration file will be
	// used.
	// Conflicts with NoInfra=true.
	// Optional.
	InfraImage string `json:"infra_image,omitempty"`
	// SharedNamespaces instructs the pod to share a set of namespaces.
	// Shared namespaces will be joined (by default) by every container
	// which joins the pod.
	// If not set and NoInfra is false, the pod will set a default set of
	// namespaces to share.
	// Conflicts with NoInfra=true.
	// Optional.
	SharedNamespaces []string `json:"shared_namespaces,omitempty"`
	// PodCreateCommand is the command used to create this pod.
	// This will be shown in the output of Inspect() on the pod, and may
	// also be used by some tools that wish to recreate the pod
	// (e.g. `podman generate systemd --new`).
	// Optional.
	PodCreateCommand []string `json:"pod_create_command,omitempty"`
}

// PodNetworkConfig contains networking configuration for a pod.
type PodNetworkConfig struct {
	// NetNS is the configuration to use for the infra container's network
	// namespace. This network will, by default, be shared with all
	// containers in the pod.
	// Cannot be set to FromContainer and FromPod.
	// Setting this to anything except default conflicts with NoInfra=true.
	// Defaults to Bridge as root and Slirp as rootless.
	// Mandatory.
	NetNS Namespace `json:"netns,omitempty"`
	// StaticIP sets a static IP for the infra container. As the infra
	// container's network is used for the entire pod by default, this will
	// thus be a static IP for the whole pod.
	// Only available if NetNS is set to Bridge (the default for root).
	// As such, conflicts with NoInfra=true by proxy.
	// Optional.
	StaticIP *net.IP `json:"static_ip,omitempty"`
	// StaticMAC sets a static MAC for the infra container. As the infra
	// container's network is used for the entire pod by default, this will
	// thus be a static MAC for the entire pod.
	// Only available if NetNS is set to Bridge (the default for root).
	// As such, conflicts with NoInfra=true by proxy.
	// Optional.
	StaticMAC *net.HardwareAddr `json:"static_mac,omitempty"`
	// PortMappings is a set of ports to map into the infra container.
	// As, by default, containers share their network with the infra
	// container, this will forward the ports to the entire pod.
	// Only available if NetNS is set to Bridge or Slirp.
	// Optional.
	PortMappings []PortMapping `json:"portmappings,omitempty"`
	// CNINetworks is a list of CNI networks that the infra container will
	// join. As, by default, containers share their network with the infra
	// container, these networks will effectively be joined by the
	// entire pod.
	// Only available when NetNS is set to Bridge, the default for root.
	// Optional.
	CNINetworks []string `json:"cni_networks,omitempty"`
	// NoManageResolvConf indicates that /etc/resolv.conf should not be
	// managed by the pod. Instead, each container will create and manage a
	// separate resolv.conf as if they had not joined a pod.
	// Conflicts with NoInfra=true and DNSServer, DNSSearch, DNSOption.
	// Optional.
	NoManageResolvConf bool `json:"no_manage_resolv_conf,omitempty"`
	// DNSServer is a set of DNS servers that will be used in the infra
	// container's resolv.conf, which will, by default, be shared with all
	// containers in the pod.
	// If not provided, the host's DNS servers will be used, unless the only
	// server set is a localhost address. As the container cannot connect to
	// the host's localhost, a default server will instead be set.
	// Conflicts with NoInfra=true.
	// Optional.
	DNSServer []net.IP `json:"dns_server,omitempty"`
	// DNSSearch is a set of DNS search domains that will be used in the
	// infra container's resolv.conf, which will, by default, be shared with
	// all containers in the pod.
	// If not provided, DNS search domains from the host's resolv.conf will
	// be used.
	// Conflicts with NoInfra=true.
	// Optional.
	DNSSearch []string `json:"dns_search,omitempty"`
	// DNSOption is a set of DNS options that will be used in the infra
	// container's resolv.conf, which will, by default, be shared with all
	// containers in the pod.
	// Conflicts with NoInfra=true.
	// Optional.
	DNSOption []string `json:"dns_option,omitempty"`
	// NoManageHosts indicates that /etc/hosts should not be managed by the
	// pod. Instead, each container will create a separate /etc/hosts as
	// they would if not in a pod.
	// Conflicts with HostAdd.
	NoManageHosts bool `json:"no_manage_hosts,omitempty"`
	// HostAdd is a set of hosts that will be added to the infra container's
	// /etc/hosts that will, by default, be shared with all containers in
	// the pod.
	// Conflicts with NoInfra=true and NoManageHosts.
	// Optional.
	HostAdd []string `json:"hostadd,omitempty"`
	// NetworkOptions are additional options for each network
	// Optional.
	NetworkOptions map[string][]string `json:"network_options,omitempty"`
}

// PodCgroupConfig contains configuration options about a pod's cgroups.
// This will be expanded in future updates to pods.
type PodCgroupConfig struct {
	// CgroupParent is the parent for the CGroup that the pod will create.
	// This pod cgroup will, in turn, be the default cgroup parent for all
	// containers in the pod.
	// Optional.
	CgroupParent string `json:"cgroup_parent,omitempty"`
}

// PodSpecGenerator describes options to create a pod
// swagger:model PodSpecGenerator
type PodSpecGenerator struct {
	PodBasicConfig
	PodNetworkConfig
	PodCgroupConfig
}

// NewPodSpecGenerator creates a new pod spec
func NewPodSpecGenerator() *PodSpecGenerator {
	return &PodSpecGenerator{}
}
