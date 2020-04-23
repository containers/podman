package abi

import (
	"sync"

	"github.com/containers/libpod/libpod"
)

// Image-related runtime linked against libpod library
type ImageEngine struct {
	Libpod *libpod.Runtime
}

// Container-related runtime linked against libpod library
type ContainerEngine struct {
	Libpod *libpod.Runtime
}

var shutdownSync sync.Once
