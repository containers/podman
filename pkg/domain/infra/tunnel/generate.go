package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/bindings/generate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) GenerateSystemd(ctx context.Context, nameOrID string, options entities.GenerateSystemdOptions) (*entities.GenerateSystemdReport, error) {
	return nil, errors.New("not implemented for tunnel")
}

func (ic *ContainerEngine) GenerateKube(ctx context.Context, nameOrID string, options entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	return generate.GenerateKube(ic.ClientCxt, nameOrID, options)
}
