package abi

import (
	"sync"

	"github.com/containers/podman/v4/libpod"
)

// Image-related runtime linked against libpod library
type ImageEngine struct {
	Libpod *libpod.Runtime
	FarmNode
}

// Container-related runtime linked against libpod library
type ContainerEngine struct {
	Libpod *libpod.Runtime
}

// Container-related runtime linked against libpod library
type SystemEngine struct {
	Libpod *libpod.Runtime
}

type FarmNode struct {
	platforms         sync.Once
	platformsErr      error
	os                string
	arch              string
	variant           string
	nativePlatforms   []string
	emulatedPlatforms []string
}

var shutdownSync sync.Once
