package abi

import (
	"context"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) HealthCheckRun(ctx context.Context, nameOrId string, options entities.HealthCheckOptions) (*define.HealthCheckResults, error) {
	status, err := ic.Libpod.HealthCheck(nameOrId)
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
