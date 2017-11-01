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

	// ErrCtrExists indicates a container with the same name or ID already
	// exists
	ErrCtrExists = errors.New("container already exists")
	// ErrPodExists indicates a pod with the same name or ID already exists
	ErrPodExists = errors.New("pod already exists")
	// ErrImageExists indicated an image with the same ID already exists
	ErrImageExists = errors.New("image already exists")

	// ErrCtrStateInvalid indicates a container is in an improper state for
	// the requested operation
	ErrCtrStateInvalid = errors.New("container state improper")

	// ErrRuntimeFinalized indicates that the runtime has already been
	// created and cannot be modified
	ErrRuntimeFinalized = errors.New("runtime has been finalized")
	// ErrCtrFinalized indicates that the container has already been created
	// and cannot be modified
	ErrCtrFinalized = errors.New("container has been finalized")
	// ErrPodFinalized indicates that the pod has already been created and
	// cannot be modified
	ErrPodFinalized = errors.New("pod has been finalized")

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

	// ErrNotImplemented indicates that the requested functionality is not
	// yet present
	ErrNotImplemented = errors.New("not yet implemented")
)
