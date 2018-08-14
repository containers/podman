package shared

import (
	"github.com/projectatomic/libpod/libpod"
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
