package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) PlayKube(ctx context.Context, path string, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	return nil, errors.New("not implemented yet")
}
