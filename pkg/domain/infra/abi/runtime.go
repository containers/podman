// +build ABISupport

package abi

import (
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
)

// Image-related runtime linked against libpod library
type ImageEngine struct {
	Libpod *libpod.Runtime
}

// Container-related runtime linked against libpod library
type ContainerEngine struct {
	entities.ContainerEngine
	Libpod *libpod.Runtime
}
