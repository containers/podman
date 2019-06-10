package shared

import (
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

const (
	PodStateStopped = "Stopped"
	PodStateRunning = "Running"
	PodStatePaused  = "Paused"
	PodStateExited  = "Exited"
	PodStateErrored = "Error"
	PodStateCreated = "Created"
)

// GetPodStatus determines the status of the pod based on the
// statuses of the containers in the pod.
// Returns a string representation of the pod status
func GetPodStatus(pod *libpod.Pod) (string, error) {
	ctrStatuses, err := pod.Status()
	if err != nil {
		return PodStateErrored, err
	}
	return CreatePodStatusResults(ctrStatuses)
}

func CreatePodStatusResults(ctrStatuses map[string]libpod.ContainerStatus) (string, error) {
	ctrNum := len(ctrStatuses)
	if ctrNum == 0 {
		return PodStateCreated, nil
	}
	statuses := map[string]int{
		PodStateStopped: 0,
		PodStateRunning: 0,
		PodStatePaused:  0,
		PodStateCreated: 0,
		PodStateErrored: 0,
	}
	for _, ctrStatus := range ctrStatuses {
		switch ctrStatus {
		case libpod.ContainerStateExited:
			fallthrough
		case libpod.ContainerStateStopped:
			statuses[PodStateStopped]++
		case libpod.ContainerStateRunning:
			statuses[PodStateRunning]++
		case libpod.ContainerStatePaused:
			statuses[PodStatePaused]++
		case libpod.ContainerStateCreated, libpod.ContainerStateConfigured:
			statuses[PodStateCreated]++
		default:
			statuses[PodStateErrored]++
		}
	}

	if statuses[PodStateRunning] > 0 {
		return PodStateRunning, nil
	} else if statuses[PodStatePaused] == ctrNum {
		return PodStatePaused, nil
	} else if statuses[PodStateStopped] == ctrNum {
		return PodStateExited, nil
	} else if statuses[PodStateStopped] > 0 {
		return PodStateStopped, nil
	} else if statuses[PodStateErrored] > 0 {
		return PodStateErrored, nil
	}
	return PodStateCreated, nil
}

// GetNamespaceOptions transforms a slice of kernel namespaces
// into a slice of pod create options. Currently, not all
// kernel namespaces are supported, and they will be returned in an error
func GetNamespaceOptions(ns []string) ([]libpod.PodCreateOption, error) {
	var options []libpod.PodCreateOption
	var erroredOptions []libpod.PodCreateOption
	for _, toShare := range ns {
		switch toShare {
		case "cgroup":
			options = append(options, libpod.WithPodCgroups())
		case "net":
			options = append(options, libpod.WithPodNet())
		case "mnt":
			return erroredOptions, errors.Errorf("Mount sharing functionality not supported on pod level")
		case "pid":
			options = append(options, libpod.WithPodPID())
		case "user":
			return erroredOptions, errors.Errorf("User sharing functionality not supported on pod level")
		case "ipc":
			options = append(options, libpod.WithPodIPC())
		case "uts":
			options = append(options, libpod.WithPodUTS())
		case "":
		case "none":
			return erroredOptions, nil
		default:
			return erroredOptions, errors.Errorf("Invalid kernel namespace to share: %s. Options are: net, pid, ipc, uts or none", toShare)
		}
	}
	return options, nil
}

// CreatePortBindings iterates ports mappings and exposed ports into a format CNI understands
func CreatePortBindings(ports []string) ([]ocicni.PortMapping, error) {
	var portBindings []ocicni.PortMapping
	// The conversion from []string to natBindings is temporary while mheon reworks the port
	// deduplication code.  Eventually that step will not be required.
	_, natBindings, err := nat.ParsePortSpecs(ports)
	if err != nil {
		return nil, err
	}
	for containerPb, hostPb := range natBindings {
		var pm ocicni.PortMapping
		pm.ContainerPort = int32(containerPb.Int())
		for _, i := range hostPb {
			var hostPort int
			var err error
			pm.HostIP = i.HostIP
			if i.HostPort == "" {
				hostPort = containerPb.Int()
			} else {
				hostPort, err = strconv.Atoi(i.HostPort)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to convert host port to integer")
				}
			}

			pm.HostPort = int32(hostPort)
			pm.Protocol = containerPb.Proto()
			portBindings = append(portBindings, pm)
		}
	}
	return portBindings, nil
}

var DefaultKernelNamespaces = "cgroup,ipc,net,uts"
