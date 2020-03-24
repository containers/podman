// +build ABISupport

package infra

import (
	"context"

	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi"
	flag "github.com/spf13/pflag"
)

// ContainerEngine Proxy will be EOL'ed after podmanV2 is separated from libpod repo

func NewLibpodRuntime(flags *flag.FlagSet, opts entities.EngineOptions) (entities.ContainerEngine, error) {
	r, err := GetRuntime(context.Background(), flags, opts)
	if err != nil {
		return nil, err
	}
	return &abi.ContainerEngine{Libpod: r}, nil
}
