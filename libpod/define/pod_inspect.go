package define

import (
	"net"
	"time"
)

// InspectPodData contains detailed information on a pod's configuration and
// state. It is used as the output of Inspect on pods.
type InspectPodData struct {
	// ID is the ID of the pod.
	ID string `json:"Id"`
	// Name is the name of the pod.
	Name string
	// Namespace is the Libpod namespace the pod is placed in.
	Namespace string `json:"Namespace,omitempty"`
	// Created is the time when the pod was created.
	Created time.Time
	// CreateCommand is the full command plus arguments of the process the
	// container has been created with.
	CreateCommand []string `json:"CreateCommand,omitempty"`
	// State represents the current state of the pod.
	State string `json:"State"`
	// Hostname is the hostname that the pod will set.
	Hostname string
	// Labels is a set of key-value labels that have been applied to the
	// pod.
	Labels map[string]string `json:"Labels,omitempty"`
	// CreateCgroup is whether this pod will create its own Cgroup to group
	// containers under.
	CreateCgroup bool
	// CgroupParent is the parent of the pod's Cgroup.
	CgroupParent string `json:"CgroupParent,omitempty"`
	// CgroupPath is the path to the pod's Cgroup.
	CgroupPath string `json:"CgroupPath,omitempty"`
	// CreateInfra is whether this pod will create an infra container to
	// share namespaces.
	CreateInfra bool
	// InfraContainerID is the ID of the pod's infra container, if one is
	// present.
	InfraContainerID string `json:"InfraContainerID,omitempty"`
	// InfraConfig is the configuration of the infra container of the pod.
	// Will only be set if CreateInfra is true.
	InfraConfig *InspectPodInfraConfig `json:"InfraConfig,omitempty"`
	// SharedNamespaces contains a list of namespaces that will be shared by
	// containers within the pod. Can only be set if CreateInfra is true.
	SharedNamespaces []string `json:"SharedNamespaces,omitempty"`
	// NumContainers is the number of containers in the pod, including the
	// infra container.
	NumContainers uint
	// Containers gives a brief summary of all containers in the pod and
	// their current status.
	Containers []InspectPodContainerInfo `json:"Containers,omitempty"`
	// CPUPeriod contains the CPU period of the pod
	CPUPeriod uint64 `json:"cpu_period,omitempty"`
	// CPUQuota contains the CPU quota of the pod
	CPUQuota int64 `json:"cpu_quota,omitempty"`
	// CPUSetCPUs contains linux specific CPU data for the pod
	CPUSetCPUs string `json:"cpuset_cpus,omitempty"`
	// Mounts contains volume related information for the pod
	Mounts []InspectMount `json:"mounts,omitempty"`
	// Devices contains the specified host devices
	Devices []InspectDevice `json:"devices,omitempty"`
	// BlkioDeviceReadBps contains the Read/Access limit for the pod's devices
	BlkioDeviceReadBps []InspectBlkioThrottleDevice `json:"device_read_bps,omitempty"`
	// VolumesFrom contains the containers that the pod inherits mounts from
	VolumesFrom []string `json:"volumes_from,omitempty"`
	// SecurityOpt contains the specified security labels and related SELinux information
	SecurityOpts []string `json:"security_opt,omitempty"`
}

// InspectPodInfraConfig contains the configuration of the pod's infra
// container.
type InspectPodInfraConfig struct {
	// PortBindings are ports that will be forwarded to the infra container
	// and then shared with the pod.
	PortBindings map[string][]InspectHostPort
	// HostNetwork is whether the infra container (and thus the whole pod)
	// will use the host's network and not create a network namespace.
	HostNetwork bool
	// StaticIP is a static IPv4 that will be assigned to the infra
	// container and then used by the pod.
	// swagger:strfmt ipv4
	StaticIP net.IP
	// StaticMAC is a static MAC address that will be assigned to the infra
	// container and then used by the pod.
	StaticMAC string
	// NoManageResolvConf indicates that the pod will not manage resolv.conf
	// and instead each container will handle their own.
	NoManageResolvConf bool
	// DNSServer is a set of DNS Servers that will be used by the infra
	// container's resolv.conf and shared with the remainder of the pod.
	DNSServer []string
	// DNSSearch is a set of DNS search domains that will be used by the
	// infra container's resolv.conf and shared with the remainder of the
	// pod.
	DNSSearch []string
	// DNSOption is a set of DNS options that will be used by the infra
	// container's resolv.conf and shared with the remainder of the pod.
	DNSOption []string
	// NoManageHosts indicates that the pod will not manage /etc/hosts and
	// instead each container will handle their own.
	NoManageHosts bool
	// HostAdd adds a number of hosts to the infra container's resolv.conf
	// which will be shared with the rest of the pod.
	HostAdd []string
	// Networks is a list of CNI networks the pod will join.
	Networks []string
	// NetworkOptions are additional options for each network
	NetworkOptions map[string][]string
	// CPUPeriod contains the CPU period of the pod
	CPUPeriod uint64 `json:"cpu_period,omitempty"`
	// CPUQuota contains the CPU quota of the pod
	CPUQuota int64 `json:"cpu_quota,omitempty"`
	// CPUSetCPUs contains linux specific CPU data for the container
	CPUSetCPUs string `json:"cpuset_cpus,omitempty"`
	// Pid is the PID namespace mode of the pod's infra container
	PidNS string `json:"pid_ns,omitempty"`
	// UserNS is the usernamespace that all the containers in the pod will join.
	UserNS string `json:"userns,omitempty"`
}

// InspectPodContainerInfo contains information on a container in a pod.
type InspectPodContainerInfo struct {
	// ID is the ID of the container.
	ID string `json:"Id"`
	// Name is the name of the container.
	Name string
	// State is the current status of the container.
	State string
}
