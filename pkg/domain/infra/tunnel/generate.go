package tunnel

import (
	"context"

	"github.com/containers/podman/v2/pkg/bindings/generate"
	"github.com/containers/podman/v2/pkg/domain/entities"
)

func (ic *ContainerEngine) GenerateSystemd(ctx context.Context, nameOrID string, options entities.GenerateSystemdOptions) (*entities.GenerateSystemdReport, error) {
	return generate.Systemd(ic.ClientCxt, nameOrID, options)
}

func (ic *ContainerEngine) GenerateKube(ctx context.Context, nameOrID string, options entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	return generate.Kube(ic.ClientCxt, nameOrID, options)
}
