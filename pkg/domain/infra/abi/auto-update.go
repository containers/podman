package abi

import (
	"context"

	"github.com/containers/libpod/pkg/autoupdate"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) AutoUpdate(ctx context.Context, options entities.AutoUpdateOptions) (*entities.AutoUpdateReport, []error) {
	// Convert the entities options to the autoupdate ones.  We can't use
	// them in the entities package as low-level packages must not leak
	// into the remote client.
	autoOpts := autoupdate.Options{Authfile: options.Authfile}
	units, failures := autoupdate.AutoUpdate(ic.Libpod, autoOpts)
	return &entities.AutoUpdateReport{Units: units}, failures
}
