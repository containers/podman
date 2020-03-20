package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/bindings/volumes"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) VolumeCreate(ctx context.Context, opts entities.VolumeCreateOptions) (*entities.IdOrNameResponse, error) {
	response, err := volumes.Create(ic.ClientCxt, opts)
	if err != nil {
		return nil, err
	}
	return &entities.IdOrNameResponse{IdOrName: response.Name}, nil
}
