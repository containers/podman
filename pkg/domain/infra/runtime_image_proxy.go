// +build ABISupport

package infra

import (
	"context"

	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi"
	"github.com/spf13/pflag"
)

// ContainerEngine Image Proxy will be EOL'ed after podmanV2 is separated from libpod repo

func NewLibpodImageRuntime(flags *pflag.FlagSet, opts entities.EngineOptions) (entities.ImageEngine, error) {
	r, err := GetRuntime(context.Background(), flags, opts)
	if err != nil {
		return nil, err
	}
	return &abi.ImageEngine{Libpod: r}, nil
}
