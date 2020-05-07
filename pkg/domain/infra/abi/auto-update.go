package abi

import (
	"context"

	"github.com/containers/libpod/pkg/autoupdate"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) AutoUpdate(ctx context.Context) (*entities.AutoUpdateReport, []error) {
	units, failures := autoupdate.AutoUpdate(ic.Libpod)
	return &entities.AutoUpdateReport{Units: units}, failures
}
