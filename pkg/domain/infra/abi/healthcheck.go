//go:build !remote

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
	report := define.HealthCheckResults{
		Status: status.String(),
	}
	return &report, nil
}
