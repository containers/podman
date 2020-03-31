package tunnel

import (
	"context"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/bindings/containers"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) HealthCheckRun(ctx context.Context, nameOrId string, options entities.HealthCheckOptions) (*define.HealthCheckResults, error) {
	return containers.RunHealthCheck(ic.ClientCxt, nameOrId)
}
