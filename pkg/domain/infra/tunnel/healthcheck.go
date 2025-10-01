package tunnel

import (
	"context"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

func (ic *ContainerEngine) HealthCheckRun(_ context.Context, nameOrID string, _ entities.HealthCheckOptions) (*define.HealthCheckResults, error) {
	return containers.RunHealthCheck(ic.ClientCtx, nameOrID, nil)
}
