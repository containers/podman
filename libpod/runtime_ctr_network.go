// +build linux

package libpod

import (
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/libpod/network"
)

func normalizeNetworkName(config *config.Config, nameOrID string) (string, error) {
	return network.NormalizeName(config, nameOrID)
}
