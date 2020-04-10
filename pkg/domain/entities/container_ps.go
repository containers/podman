package entities

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
)

// Listcontainer describes a container suitable for listing
type ListContainer struct {
	// Container command
	Cmd []string
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
	ContainerNames []string
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
	PortMappings []ocicni.PortMapping
	// Size of the container rootfs.  Requires the size boolean to be true
	ContainerSize *shared.ContainerSize
	// Time when container started
	StartedAt int64
	// State of container
	ContainerState string
}

// State returns the container state in human duration
func (l ListContainer) State() string {
	var state string
	switch l.ContainerState {
	case "running":
		t := units.HumanDuration(time.Since(time.Unix(l.StartedAt, 0)))
		state = "Up " + t + " ago"
	case "configured":
		state = "Created"
	case "exited", "stopped":
		t := units.HumanDuration(time.Since(time.Unix(l.ExitedAt, 0)))
		state = fmt.Sprintf("Exited (%d) %s ago", l.ExitCode, t)
	default:
		state = l.ContainerState
	}
	return state
}

// Command returns the container command in string format
func (l ListContainer) Command() string {
	return strings.Join(l.Cmd, " ")
}

// Size returns the rootfs and virtual sizes in human duration in
// and output form (string) suitable for ps
func (l ListContainer) Size() string {
	virt := units.HumanSizeWithPrecision(float64(l.ContainerSize.RootFsSize), 3)
	s := units.HumanSizeWithPrecision(float64(l.ContainerSize.RwSize), 3)
	return fmt.Sprintf("%s (virtual %s)", s, virt)
}

// Names returns the container name in string format
func (l ListContainer) Names() string {
	return l.ContainerNames[0]
}

// Ports converts from Portmappings to the string form
// required by ps
func (l ListContainer) Ports() string {
	if len(l.PortMappings) < 1 {
		return ""
	}
	return portsToString(l.PortMappings)
}

// CreatedAt returns the container creation time in string format.  podman
// and docker both return a timestamped value for createdat
func (l ListContainer) CreatedAt() string {
	return time.Unix(l.Created, 0).String()
}

// CreateHuman allows us to output the created time in human readable format
func (l ListContainer) CreatedHuman() string {
	return units.HumanDuration(time.Since(time.Unix(l.Created, 0))) + " ago"
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

// SortContainers helps us set-up ability to sort by createTime
type SortContainers []*libpod.Container

func (a SortContainers) Len() int      { return len(a) }
func (a SortContainers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type SortCreateTime struct{ SortContainers }

func (a SortCreateTime) Less(i, j int) bool {
	return a.SortContainers[i].CreatedTime().Before(a.SortContainers[j].CreatedTime())
}

type SortListContainers []ListContainer

func (a SortListContainers) Len() int      { return len(a) }
func (a SortListContainers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type psSortedCommand struct{ SortListContainers }

func (a psSortedCommand) Less(i, j int) bool {
	return strings.Join(a.SortListContainers[i].Cmd, " ") < strings.Join(a.SortListContainers[j].Cmd, " ")
}

type psSortedId struct{ SortListContainers }

func (a psSortedId) Less(i, j int) bool {
	return a.SortListContainers[i].ID < a.SortListContainers[j].ID
}

type psSortedImage struct{ SortListContainers }

func (a psSortedImage) Less(i, j int) bool {
	return a.SortListContainers[i].Image < a.SortListContainers[j].Image
}

type psSortedNames struct{ SortListContainers }

func (a psSortedNames) Less(i, j int) bool {
	return a.SortListContainers[i].ContainerNames[0] < a.SortListContainers[j].ContainerNames[0]
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
	return a.SortListContainers[i].ContainerState < a.SortListContainers[j].ContainerState
}

type psSortedSize struct{ SortListContainers }

func (a psSortedSize) Less(i, j int) bool {
	if a.SortListContainers[i].ContainerSize == nil || a.SortListContainers[j].ContainerSize == nil {
		return false
	}
	return a.SortListContainers[i].ContainerSize.RootFsSize < a.SortListContainers[j].ContainerSize.RootFsSize
}

type PsSortedCreateTime struct{ SortListContainers }

func (a PsSortedCreateTime) Less(i, j int) bool {
	return a.SortListContainers[i].Created < a.SortListContainers[j].Created
}

func SortPsOutput(sortBy string, psOutput SortListContainers) (SortListContainers, error) {
	switch sortBy {
	case "id":
		sort.Sort(psSortedId{psOutput})
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

// portsToString converts the ports used to a string of the from "port1, port2"
// and also groups a continuous list of ports into a readable format.
func portsToString(ports []ocicni.PortMapping) string {
	type portGroup struct {
		first int32
		last  int32
	}
	var portDisplay []string
	if len(ports) == 0 {
		return ""
	}
	//Sort the ports, so grouping continuous ports become easy.
	sort.Slice(ports, func(i, j int) bool {
		return comparePorts(ports[i], ports[j])
	})

	// portGroupMap is used for grouping continuous ports.
	portGroupMap := make(map[string]*portGroup)
	var groupKeyList []string

	for _, v := range ports {

		hostIP := v.HostIP
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		// If hostPort and containerPort are not same, consider as individual port.
		if v.ContainerPort != v.HostPort {
			portDisplay = append(portDisplay, fmt.Sprintf("%s:%d->%d/%s", hostIP, v.HostPort, v.ContainerPort, v.Protocol))
			continue
		}

		portMapKey := fmt.Sprintf("%s/%s", hostIP, v.Protocol)

		portgroup, ok := portGroupMap[portMapKey]
		if !ok {
			portGroupMap[portMapKey] = &portGroup{first: v.ContainerPort, last: v.ContainerPort}
			// This list is required to traverse portGroupMap.
			groupKeyList = append(groupKeyList, portMapKey)
			continue
		}

		if portgroup.last == (v.ContainerPort - 1) {
			portgroup.last = v.ContainerPort
			continue
		}
	}
	// For each portMapKey, format group list and appned to output string.
	for _, portKey := range groupKeyList {
		group := portGroupMap[portKey]
		portDisplay = append(portDisplay, formatGroup(portKey, group.first, group.last))
	}
	return strings.Join(portDisplay, ", ")
}

func comparePorts(i, j ocicni.PortMapping) bool {
	if i.ContainerPort != j.ContainerPort {
		return i.ContainerPort < j.ContainerPort
	}

	if i.HostIP != j.HostIP {
		return i.HostIP < j.HostIP
	}

	if i.HostPort != j.HostPort {
		return i.HostPort < j.HostPort
	}

	return i.Protocol < j.Protocol
}

// formatGroup returns the group as <IP:startPort:lastPort->startPort:lastPort/Proto>
// e.g 0.0.0.0:1000-1006->1000-1006/tcp.
func formatGroup(key string, start, last int32) string {
	parts := strings.Split(key, "/")
	groupType := parts[0]
	var ip string
	if len(parts) > 1 {
		ip = parts[0]
		groupType = parts[1]
	}
	group := strconv.Itoa(int(start))
	if start != last {
		group = fmt.Sprintf("%s-%d", group, last)
	}
	if ip != "" {
		group = fmt.Sprintf("%s:%s->%s", ip, group, group)
	}
	return fmt.Sprintf("%s/%s", group, groupType)
}
