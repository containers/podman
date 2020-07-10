package abi

import (
	"context"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
)

func (ic *ContainerEngine) HealthCheckRun(ctx context.Context, nameOrID string, options entities.HealthCheckOptions) (*define.HealthCheckResults, error) {
	status, err := ic.Libpod.HealthCheck(nameOrID)
	if err != nil {
		return nil, err
	}
	hcStatus := define.HealthCheckUnhealthy
	if status == define.HealthCheckSuccess {
		hcStatus = define.HealthCheckHealthy
	}
	report := define.HealthCheckResults{
		Status: hcStatus,
	}
	return &report, nil
}
