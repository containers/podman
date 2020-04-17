package registry

import (
	"github.com/containers/libpod/pkg/domain/entities"
)

func IsRemote() bool {
	return podmanOptions.EngineMode == entities.TunnelMode
}
