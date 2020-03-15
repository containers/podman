package libpod

import (
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/cri-o/ocicni/pkg/ocicni"
)

// Listcontainer describes a container suitable for listing
type ListContainer struct {
	// Container command
	Command []string
	// Container creation time
	Created int64
	// If container has exited/stopped
	Exited bool
	// Time container exited
	ExitedAt int64
	// If container has exited, the return code from the command
	ExitCode int32
	// The unique identifier for the container
	ID string `json:"Id"`
	// Container image
	Image string
	// If this container is a Pod infra container
	IsInfra bool
	// Labels for container
	Labels map[string]string
	// User volume mounts
	Mounts []string
	// The names assigned to the container
	Names []string
	// Namespaces the container belongs to.  Requires the
	// namespace boolean to be true
	Namespaces ListContainerNamespaces
	// The process id of the container
	Pid int
	// If the container is part of Pod, the Pod ID. Requires the pod
	// boolean to be set
	Pod string
	// If the container is part of Pod, the Pod name. Requires the pod
	// boolean to be set
	PodName string
	// Port mappings
	Ports []ocicni.PortMapping
	// Size of the container rootfs.  Requires the size boolean to be true
	Size *shared.ContainerSize
	// Time when container started
	StartedAt int64
	// State of container
	State string
}

// ListContainer Namespaces contains the identifiers of the container's Linux namespaces
type ListContainerNamespaces struct {
	// Mount namespace
	MNT string `json:"Mnt,omitempty"`
	// Cgroup namespace
	Cgroup string `json:"Cgroup,omitempty"`
	// IPC namespace
	IPC string `json:"Ipc,omitempty"`
	// Network namespace
	NET string `json:"Net,omitempty"`
	// PID namespace
	PIDNS string `json:"Pidns,omitempty"`
	// UTS namespace
	UTS string `json:"Uts,omitempty"`
	// User namespace
	User string `json:"User,omitempty"`
}

// sortContainers helps us set-up ability to sort by createTime
type sortContainers []*libpod.Container

func (a sortContainers) Len() int      { return len(a) }
func (a sortContainers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type psSortCreateTime struct{ sortContainers }

func (a psSortCreateTime) Less(i, j int) bool {
	return a.sortContainers[i].CreatedTime().Before(a.sortContainers[j].CreatedTime())
}
