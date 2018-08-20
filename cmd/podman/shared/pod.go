package shared

import (
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
)

const (
	stopped = "Stopped"
	running = "Running"
	paused  = "Paused"
	exited  = "Exited"
	errored = "Error"
	created = "Created"
)

// GetPodStatus determines the status of the pod based on the
// statuses of the containers in the pod.
// Returns a string representation of the pod status
func GetPodStatus(pod *libpod.Pod) (string, error) {
	ctrStatuses, err := pod.Status()
	if err != nil {
		return errored, err
	}
	ctrNum := len(ctrStatuses)
	if ctrNum == 0 {
		return created, nil
	}
	statuses := map[string]int{
		stopped: 0,
		running: 0,
		paused:  0,
		created: 0,
		errored: 0,
	}
	for _, ctrStatus := range ctrStatuses {
		switch ctrStatus {
		case libpod.ContainerStateStopped:
			statuses[stopped]++
		case libpod.ContainerStateRunning:
			statuses[running]++
		case libpod.ContainerStatePaused:
			statuses[paused]++
		case libpod.ContainerStateCreated, libpod.ContainerStateConfigured:
			statuses[created]++
		default:
			statuses[errored]++
		}
	}

	if statuses[running] > 0 {
		return running, nil
	} else if statuses[paused] == ctrNum {
		return paused, nil
	} else if statuses[stopped] == ctrNum {
		return exited, nil
	} else if statuses[stopped] > 0 {
		return stopped, nil
	} else if statuses[errored] > 0 {
		return errored, nil
	}
	return created, nil
}

// GetNamespaceOptions transforms a slice of kernel namespaces
// into a slice of pod create options. Currently, not all
// kernel namespaces are supported, and they will be returned in an error
func GetNamespaceOptions(ns []string) ([]libpod.PodCreateOption, error) {
	var options []libpod.PodCreateOption
	var erroredOptions []libpod.PodCreateOption
	for _, toShare := range ns {
		switch toShare {
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
