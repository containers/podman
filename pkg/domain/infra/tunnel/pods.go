package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/bindings/pods"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) PodExists(ctx context.Context, nameOrId string) (*entities.BoolReport, error) {
	exists, err := pods.Exists(ic.ClientCxt, nameOrId)
	return &entities.BoolReport{Value: exists}, err
}
