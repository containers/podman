package tunnel

import (
	"context"

	"github.com/containers/libpod/v2/libpod/define"
	"github.com/containers/libpod/v2/pkg/bindings/containers"
	"github.com/containers/libpod/v2/pkg/domain/entities"
)

func (ic *ContainerEngine) HealthCheckRun(ctx context.Context, nameOrID string, options entities.HealthCheckOptions) (*define.HealthCheckResults, error) {
	return containers.RunHealthCheck(ic.ClientCxt, nameOrID)
}
