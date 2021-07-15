package abi

import (
	"context"

	"github.com/containers/podman/v3/pkg/autoupdate"
	"github.com/containers/podman/v3/pkg/domain/entities"
)

func (ic *ContainerEngine) AutoUpdate(ctx context.Context, options entities.AutoUpdateOptions) ([]*entities.AutoUpdateReport, []error) {
	return autoupdate.AutoUpdate(ctx, ic.Libpod, options)
}
