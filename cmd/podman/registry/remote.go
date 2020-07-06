package registry

import (
	"github.com/containers/libpod/v2/pkg/domain/entities"
)

func IsRemote() bool {
	return podmanOptions.EngineMode == entities.TunnelMode
}
