package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) ContainerExists(ctx context.Context, nameOrId string) (*entities.BoolReport, error) {
	exists, err := containers.Exists(ctx, nameOrId)
	return &entities.BoolReport{Value: exists}, err
}

func (ic *ContainerEngine) ContainerWait(ctx context.Context, namesOrIds []string, options entities.WaitOptions) ([]entities.WaitReport, error) {
	return nil, nil
}

func (r *ContainerEngine) ContainerDelete(ctx context.Context, opts entities.ContainerDeleteOptions) (*entities.ContainerDeleteReport, error) {
	panic("implement me")
}

func (r *ContainerEngine) ContainerPrune(ctx context.Context) (*entities.ContainerPruneReport, error) {
	panic("implement me")
}
