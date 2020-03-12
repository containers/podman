// +build ABISupport

package infra

import (
	"context"

	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi"
	"github.com/spf13/pflag"
)

// ContainerEngine Image Proxy will be EOL'ed after podmanV2 is separated from libpod repo

func NewLibpodImageRuntime(flags pflag.FlagSet, opts entities.EngineFlags) (entities.ImageEngine, error) {
	r, err := GetRuntime(context.Background(), flags, opts)
	if err != nil {
		return nil, err
	}
	return &abi.ImageEngine{Libpod: r}, nil
}

func (ir *runtime) ShutdownImageRuntime(force bool) error {
	return ir.Libpod.Shutdown(force)
}
