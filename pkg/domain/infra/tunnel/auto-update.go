package tunnel

import (
	"context"

	autoupdate "github.com/containers/podman/v5/pkg/bindings/auto-update"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"go.podman.io/image/v5/types"
)

func (ic *ContainerEngine) AutoUpdate(ctx context.Context, opts entities.AutoUpdateOptions) ([]*entities.AutoUpdateReport, []error) {
	options := new(autoupdate.AutoUpdateOptions).WithAuthfile(opts.Authfile).WithDryRun(opts.DryRun).WithRollback(opts.Rollback)
	if s := opts.InsecureSkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithInsecureSkipTLSVerify(true)
		} else {
			options.WithInsecureSkipTLSVerify(false)
		}
	}

	return autoupdate.AutoUpdate(ic.ClientCtx, options)
}
