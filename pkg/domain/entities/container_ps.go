package entities

import (
	"sort"
	"strings"
	"time"

	"github.com/containers/podman/v3/pkg/ps/define"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
)

// Listcontainer describes a container suitable for listing
type ListContainer struct {
	// AutoRemove
	AutoRemove bool
	// Container command
	Command []string
	// Container creation time
	Created time.Time
	// Human readable container creation time.
	CreatedAt string
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
	// Container image ID
	ImageID string
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
	// The network names assigned to the container
	Networks []string
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
	Size *define.ContainerSize
	// Time when container started
	StartedAt int64
	// State of container
	State string
	// Status is a human-readable approximation of a duration for json output
	Status string
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

type SortListContainers []ListContainer

func (a SortListContainers) Len() int      { return len(a) }
func (a SortListContainers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type psSortedCommand struct{ SortListContainers }

func (a psSortedCommand) Less(i, j int) bool {
	return strings.Join(a.SortListContainers[i].Command, " ") < strings.Join(a.SortListContainers[j].Command, " ")
}

type psSortedID struct{ SortListContainers }

func (a psSortedID) Less(i, j int) bool {
	return a.SortListContainers[i].ID < a.SortListContainers[j].ID
}

type psSortedImage struct{ SortListContainers }

func (a psSortedImage) Less(i, j int) bool {
	return a.SortListContainers[i].Image < a.SortListContainers[j].Image
}

type psSortedNames struct{ SortListContainers }

func (a psSortedNames) Less(i, j int) bool {
	return a.SortListContainers[i].Names[0] < a.SortListContainers[j].Names[0]
}

type psSortedPod struct{ SortListContainers }

func (a psSortedPod) Less(i, j int) bool {
	return a.SortListContainers[i].Pod < a.SortListContainers[j].Pod
}

type psSortedRunningFor struct{ SortListContainers }

func (a psSortedRunningFor) Less(i, j int) bool {
	return a.SortListContainers[i].StartedAt < a.SortListContainers[j].StartedAt
}

type psSortedStatus struct{ SortListContainers }

func (a psSortedStatus) Less(i, j int) bool {
	return a.SortListContainers[i].State < a.SortListContainers[j].State
}

type psSortedSize struct{ SortListContainers }

func (a psSortedSize) Less(i, j int) bool {
	if a.SortListContainers[i].Size == nil || a.SortListContainers[j].Size == nil {
		return false
	}
	return a.SortListContainers[i].Size.RootFsSize < a.SortListContainers[j].Size.RootFsSize
}

type PsSortedCreateTime struct{ SortListContainers }

func (a PsSortedCreateTime) Less(i, j int) bool {
	return a.SortListContainers[i].Created.Before(a.SortListContainers[j].Created)
}

func SortPsOutput(sortBy string, psOutput SortListContainers) (SortListContainers, error) {
	switch sortBy {
	case "id":
		sort.Sort(psSortedID{psOutput})
	case "image":
		sort.Sort(psSortedImage{psOutput})
	case "command":
		sort.Sort(psSortedCommand{psOutput})
	case "runningfor":
		sort.Sort(psSortedRunningFor{psOutput})
	case "status":
		sort.Sort(psSortedStatus{psOutput})
	case "size":
		sort.Sort(psSortedSize{psOutput})
	case "names":
		sort.Sort(psSortedNames{psOutput})
	case "created":
		sort.Sort(PsSortedCreateTime{psOutput})
	case "pod":
		sort.Sort(psSortedPod{psOutput})
	default:
		return nil, errors.Errorf("invalid option for --sort, options are: command, created, id, image, names, runningfor, size, or status")
	}
	return psOutput, nil
}

func (l ListContainer) CGROUPNS() string {
	return l.Namespaces.Cgroup
}

func (l ListContainer) IPC() string {
	return l.Namespaces.IPC
}

func (l ListContainer) MNT() string {
	return l.Namespaces.MNT
}

func (l ListContainer) NET() string {
	return l.Namespaces.NET
}

func (l ListContainer) PIDNS() string {
	return l.Namespaces.PIDNS
}

func (l ListContainer) USERNS() string {
	return l.Namespaces.User
}

func (l ListContainer) UTS() string {
	return l.Namespaces.UTS
}
