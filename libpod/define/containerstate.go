package define

import "github.com/pkg/errors"

// ContainerStatus represents the current state of a container
type ContainerStatus int

const (
	// ContainerStateUnknown indicates that the container is in an error
	// state where information about it cannot be retrieved
	ContainerStateUnknown ContainerStatus = iota
	// ContainerStateConfigured indicates that the container has had its
	// storage configured but it has not been created in the OCI runtime
	ContainerStateConfigured ContainerStatus = iota
	// ContainerStateCreated indicates the container has been created in
	// the OCI runtime but not started
	ContainerStateCreated ContainerStatus = iota
	// ContainerStateRunning indicates the container is currently executing
	ContainerStateRunning ContainerStatus = iota
	// ContainerStateStopped indicates that the container was running but has
	// exited
	ContainerStateStopped ContainerStatus = iota
	// ContainerStatePaused indicates that the container has been paused
	ContainerStatePaused ContainerStatus = iota
	// ContainerStateExited indicates the the container has stopped and been
	// cleaned up
	ContainerStateExited ContainerStatus = iota
	// ContainerStateRemoving indicates the container is in the process of
	// being removed.
	ContainerStateRemoving ContainerStatus = iota
)

// ContainerStatus returns a string representation for users
// of a container state
func (t ContainerStatus) String() string {
	switch t {
	case ContainerStateUnknown:
		return "unknown"
	case ContainerStateConfigured:
		return "configured"
	case ContainerStateCreated:
		return "created"
	case ContainerStateRunning:
		return "running"
	case ContainerStateStopped:
		return "stopped"
	case ContainerStatePaused:
		return "paused"
	case ContainerStateExited:
		return "exited"
	case ContainerStateRemoving:
		return "removing"
	}
	return "bad state"
}

// StringToContainerStatus converts a string representation of a containers
// status into an actual container status type
func StringToContainerStatus(status string) (ContainerStatus, error) {
	switch status {
	case ContainerStateUnknown.String():
		return ContainerStateUnknown, nil
	case ContainerStateConfigured.String():
		return ContainerStateConfigured, nil
	case ContainerStateCreated.String():
		return ContainerStateCreated, nil
	case ContainerStateRunning.String():
		return ContainerStateRunning, nil
	case ContainerStateStopped.String():
		return ContainerStateStopped, nil
	case ContainerStatePaused.String():
		return ContainerStatePaused, nil
	case ContainerStateExited.String():
		return ContainerStateExited, nil
	case ContainerStateRemoving.String():
		return ContainerStateRemoving, nil
	default:
		return ContainerStateUnknown, errors.Wrapf(ErrInvalidArg, "unknown container state: %s", status)
	}
}
