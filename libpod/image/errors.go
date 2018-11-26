package image

import (
	"errors"
)

// Copied directly from libpod errors to avoid circular imports
var (
	// ErrNoSuchCtr indicates the requested container does not exist
	ErrNoSuchCtr = errors.New("no such container")
	// ErrNoSuchPod indicates the requested pod does not exist
	ErrNoSuchPod = errors.New("no such pod")
	// ErrNoSuchImage indicates the requested image does not exist
	ErrNoSuchImage = errors.New("no such image")
)
