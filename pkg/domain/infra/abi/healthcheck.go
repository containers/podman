package abi

import (
	"context"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

func (ic *ContainerEngine) HealthCheckRun(ctx context.Context, nameOrID string, options entities.HealthCheckOptions) (*define.HealthCheckResults, error) {
	status, err := ic.Libpod.HealthCheck(ctx, nameOrID)
	if err != nil {
		return nil, err
	}
	hcStatus := define.HealthCheckUnhealthy
	if status == define.HealthCheckSuccess {
		hcStatus = define.HealthCheckHealthy
	} else if status == define.HealthCheckStartup {
		hcStatus = define.HealthCheckStarting
	}
	report := define.HealthCheckResults{
		Status: hcStatus,
	}
	return &report, nil
}
