package tunnel

import (
	"context"

	"github.com/containers/podman/v6/libpod/define"
	"github.com/containers/podman/v6/pkg/bindings/containers"
	"github.com/containers/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) HealthCheckRun(_ context.Context, nameOrID string, _ entities.HealthCheckOptions) (*define.HealthCheckResults, error) {
	return containers.RunHealthCheck(ic.ClientCtx, nameOrID, nil)
}
