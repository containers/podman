package libpod

import (
	"errors"
)

var (
	// ErrNoSuchCtr indicates the requested container does not exist
	ErrNoSuchCtr = errors.New("no such container")
	// ErrNoSuchPod indicates the requested pod does not exist
	ErrNoSuchPod = errors.New("no such pod")
	// ErrNoSuchImage indicates the requested image does not exist
	ErrNoSuchImage = errors.New("no such image")
	// ErrNoSuchVolume indicates the requested volume does not exist
	ErrNoSuchVolume = errors.New("no such volume")

	// ErrCtrExists indicates a container with the same name or ID already
	// exists
	ErrCtrExists = errors.New("container already exists")
	// ErrPodExists indicates a pod with the same name or ID already exists
	ErrPodExists = errors.New("pod already exists")
	// ErrImageExists indicates an image with the same ID already exists
	ErrImageExists = errors.New("image already exists")
	// ErrVolumeExists indicates a volume with the same name already exists
	ErrVolumeExists = errors.New("volume already exists")

	// ErrCtrStateInvalid indicates a container is in an improper state for
	// the requested operation
	ErrCtrStateInvalid = errors.New("container state improper")
	// ErrVolumeBeingUsed indicates that a volume is being used by at least one container
	ErrVolumeBeingUsed = errors.New("volume is being used")

	// ErrRuntimeFinalized indicates that the runtime has already been
	// created and cannot be modified
	ErrRuntimeFinalized = errors.New("runtime has been finalized")
	// ErrCtrFinalized indicates that the container has already been created
	// and cannot be modified
	ErrCtrFinalized = errors.New("container has been finalized")
	// ErrPodFinalized indicates that the pod has already been created and
	// cannot be modified
	ErrPodFinalized = errors.New("pod has been finalized")
	// ErrVolumeFinalized indicates that the volume has already been created and
	// cannot be modified
	ErrVolumeFinalized = errors.New("volume has been finalized")

	// ErrInvalidArg indicates that an invalid argument was passed
	ErrInvalidArg = errors.New("invalid argument")
	// ErrEmptyID indicates that an empty ID was passed
	ErrEmptyID = errors.New("name or ID cannot be empty")

	// ErrInternal indicates an internal library error
	ErrInternal = errors.New("internal libpod error")

	// ErrRuntimeStopped indicates that the runtime has already been shut
	// down and no further operations can be performed on it
	ErrRuntimeStopped = errors.New("runtime has already been stopped")
	// ErrCtrStopped indicates that the requested container is not running
	// and the requested operation cannot be performed until it is started
	ErrCtrStopped = errors.New("container is stopped")

	// ErrCtrRemoved indicates that the container has already been removed
	// and no further operations can be performed on it
	ErrCtrRemoved = errors.New("container has already been removed")
	// ErrPodRemoved indicates that the pod has already been removed and no
	// further operations can be performed on it
	ErrPodRemoved = errors.New("pod has already been removed")
	// ErrVolumeRemoved indicates that the volume has already been removed and
	// no further operations can be performed on it
	ErrVolumeRemoved = errors.New("volume has already been removed")

	// ErrDBClosed indicates that the connection to the state database has
	// already been closed
	ErrDBClosed = errors.New("database connection already closed")
	// ErrDBBadConfig indicates that the database has a different schema or
	// was created by a libpod with a different config
	ErrDBBadConfig = errors.New("database configuration mismatch")

	// ErrNSMismatch indicates that the requested pod or container is in a
	// different namespace and cannot be accessed or modified.
	ErrNSMismatch = errors.New("target is in a different namespace")

	// ErrNotImplemented indicates that the requested functionality is not
	// yet present
	ErrNotImplemented = errors.New("not yet implemented")

	// ErrOSNotSupported indicates the function is not available on the particular
	// OS.
	ErrOSNotSupported = errors.New("No support for this OS yet")
)
