package define

import (
	"strings"
)

type LibpodError string

func (e LibpodError) Error() string {
	return string(e)
}

func (e LibpodError) Is(target error) bool {
	return strings.HasPrefix(target.Error(), string(e))
}

var (
	// ErrNoSuchCtr indicates the requested container does not exist
	ErrNoSuchCtr = LibpodError("no such container")

	// ErrNoSuchPod indicates the requested pod does not exist
	ErrNoSuchPod = LibpodError("no such pod")

	// ErrNoSuchImage indicates the requested image does not exist
	ErrNoSuchImage = LibpodError("no such image")

	// ErrNoSuchTag indicates the requested image tag does not exist
	ErrNoSuchTag = LibpodError("no such tag")

	// ErrNoSuchVolume indicates the requested volume does not exist
	ErrNoSuchVolume = LibpodError("no such volume")

	// ErrNoSuchNetwork indicates the requested network does not exist
	ErrNoSuchNetwork = LibpodError("network not found")

	// ErrNoSuchExecSession indicates that the requested exec session does
	// not exist.
	ErrNoSuchExecSession = LibpodError("no such exec session")

	// ErrCtrExists indicates a container with the same name or ID already
	// exists
	ErrCtrExists = LibpodError("container already exists")
	// ErrPodExists indicates a pod with the same name or ID already exists
	ErrPodExists = LibpodError("pod already exists")
	// ErrImageExists indicates an image with the same ID already exists
	ErrImageExists = LibpodError("image already exists")
	// ErrVolumeExists indicates a volume with the same name already exists
	ErrVolumeExists = LibpodError("volume already exists")
	// ErrExecSessionExists indicates an exec session with the same ID
	// already exists.
	ErrExecSessionExists = LibpodError("exec session already exists")

	// ErrCtrStateInvalid indicates a container is in an improper state for
	// the requested operation
	ErrCtrStateInvalid = LibpodError("container state improper")
	// ErrExecSessionStateInvalid indicates that an exec session is in an
	// improper state for the requested operation
	ErrExecSessionStateInvalid = LibpodError("exec session state improper")
	// ErrVolumeBeingUsed indicates that a volume is being used by at least one container
	ErrVolumeBeingUsed = LibpodError("volume is being used")

	// ErrRuntimeFinalized indicates that the runtime has already been
	// created and cannot be modified
	ErrRuntimeFinalized = LibpodError("runtime has been finalized")
	// ErrCtrFinalized indicates that the container has already been created
	// and cannot be modified
	ErrCtrFinalized = LibpodError("container has been finalized")
	// ErrPodFinalized indicates that the pod has already been created and
	// cannot be modified
	ErrPodFinalized = LibpodError("pod has been finalized")
	// ErrVolumeFinalized indicates that the volume has already been created and
	// cannot be modified
	ErrVolumeFinalized = LibpodError("volume has been finalized")

	// ErrInvalidArg indicates that an invalid argument was passed
	ErrInvalidArg = LibpodError("invalid argument")
	// ErrEmptyID indicates that an empty ID was passed
	ErrEmptyID = LibpodError("name or ID cannot be empty")

	// ErrInternal indicates an internal library error
	ErrInternal = LibpodError("internal libpod error")

	// ErrPodPartialFail indicates that a pod operation was only partially
	// successful, and some containers within the pod failed.
	ErrPodPartialFail = LibpodError("some containers failed")

	// ErrDetach indicates that an attach session was manually detached by
	// the user.
	ErrDetach = LibpodError("detached from container")

	// ErrWillDeadlock indicates that the requested operation will cause a
	// deadlock. This is usually caused by upgrade issues, and is resolved
	// by renumbering the locks.
	ErrWillDeadlock = LibpodError("deadlock due to lock mismatch")

	// ErrNoCgroups indicates that the container does not have its own
	// CGroup.
	ErrNoCgroups = LibpodError("this container does not have a cgroup")
	// ErrNoLogs indicates that this container is not creating a log so log
	// operations cannot be performed on it
	ErrNoLogs = LibpodError("this container is not logging output")

	// ErrRootless indicates that the given command cannot but run without
	// root.
	ErrRootless = LibpodError("operation requires root privileges")

	// ErrRuntimeStopped indicates that the runtime has already been shut
	// down and no further operations can be performed on it
	ErrRuntimeStopped = LibpodError("runtime has already been stopped")
	// ErrCtrStopped indicates that the requested container is not running
	// and the requested operation cannot be performed until it is started
	ErrCtrStopped = LibpodError("container is stopped")

	// ErrCtrRemoved indicates that the container has already been removed
	// and no further operations can be performed on it
	ErrCtrRemoved = LibpodError("container has already been removed")
	// ErrPodRemoved indicates that the pod has already been removed and no
	// further operations can be performed on it
	ErrPodRemoved = LibpodError("pod has already been removed")
	// ErrVolumeRemoved indicates that the volume has already been removed and
	// no further operations can be performed on it
	ErrVolumeRemoved = LibpodError("volume has already been removed")
	// ErrExecSessionRemoved indicates that the exec session has already
	// been removed and no further operations can be performed on it.
	ErrExecSessionRemoved = LibpodError("exec session has already been removed")

	// ErrDBClosed indicates that the connection to the state database has
	// already been closed
	ErrDBClosed = LibpodError("database connection already closed")
	// ErrDBBadConfig indicates that the database has a different schema or
	// was created by a libpod with a different config
	ErrDBBadConfig = LibpodError("database configuration mismatch")

	// ErrNSMismatch indicates that the requested pod or container is in a
	// different namespace and cannot be accessed or modified.
	ErrNSMismatch = LibpodError("target is in a different namespace")

	// ErrNotImplemented indicates that the requested functionality is not
	// yet present
	ErrNotImplemented = LibpodError("not yet implemented")

	// ErrOSNotSupported indicates the function is not available on the particular
	// OS.
	ErrOSNotSupported = LibpodError("no support for this OS yet")

	// ErrOCIRuntime indicates a generic error from the OCI runtime
	ErrOCIRuntime = LibpodError("OCI runtime error")

	// ErrOCIRuntimePermissionDenied indicates the OCI runtime attempted to invoke a command that returned
	// a permission denied error
	ErrOCIRuntimePermissionDenied = LibpodError("OCI runtime permission denied error")

	// ErrOCIRuntimeNotFound indicates the OCI runtime attempted to invoke a command
	// that was not found
	ErrOCIRuntimeNotFound = LibpodError("OCI runtime command not found error")

	// ErrOCIRuntimeUnavailable indicates that the OCI runtime associated to a container
	// could not be found in the configuration
	ErrOCIRuntimeUnavailable = LibpodError("OCI runtime not available in the current configuration")

	// ErrConmonOutdated indicates the version of conmon found (whether via the configuration or $PATH)
	// is out of date for the current podman version
	ErrConmonOutdated = LibpodError("outdated conmon version")
	// ErrConmonDead indicates that the container's conmon process has been
	// killed, preventing normal operation.
	ErrConmonDead = LibpodError("conmon process killed")

	// ErrImageInUse indicates the requested operation failed because the image was in use
	ErrImageInUse = LibpodError("image is being used")

	// ErrNetworkOnPodContainer indicates the user wishes to alter network attributes on a container
	// in a pod.  This cannot be done as the infra container has all the network information
	ErrNetworkOnPodContainer = LibpodError("network cannot be configured when it is shared with a pod")

	// ErrNetworkInUse indicates the requested operation failed because the network was in use
	ErrNetworkInUse = LibpodError("network is being used")

	// ErrStoreNotInitialized indicates that the container storage was never
	// initialized.
	ErrStoreNotInitialized = LibpodError("the container storage was never initialized")
)
