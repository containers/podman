package tunnel

import (
	"context"

	"github.com/containers/podman/v2/pkg/bindings/play"
	"github.com/containers/podman/v2/pkg/domain/entities"
)

func (ic *ContainerEngine) PlayKube(ctx context.Context, path string, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	return play.Kube(ic.ClientCxt, path, options)
}
