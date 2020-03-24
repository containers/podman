package utils

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
)

func IsRemote() bool {
	return registry.EngineOpts.EngineMode == entities.TunnelMode
}
