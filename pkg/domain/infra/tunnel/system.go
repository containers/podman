package tunnel

import (
	"context"
	"errors"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings/system"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

func (ic *ContainerEngine) Info(ctx context.Context) (*define.Info, error) {
	return system.Info(ic.ClientCtx, nil)
}

func (ic *ContainerEngine) SetupRootless(_ context.Context, noMoveProcess bool) error {
	panic(errors.New("rootless engine mode is not supported when tunneling"))
}

// SystemPrune prunes unused data from the system.
func (ic *ContainerEngine) SystemPrune(ctx context.Context, opts entities.SystemPruneOptions) (*entities.SystemPruneReport, error) {
	options := new(system.PruneOptions).WithAll(opts.All).WithVolumes(opts.Volume).WithFilters(opts.Filters).WithExternal(opts.External)
	return system.Prune(ic.ClientCtx, options)
}

func (ic *ContainerEngine) SystemDf(ctx context.Context, options entities.SystemDfOptions) (*entities.SystemDfReport, error) {
	return system.DiskUsage(ic.ClientCtx, nil)
}

func (ic *ContainerEngine) Unshare(ctx context.Context, args []string, options entities.SystemUnshareOptions) error {
	return errors.New("unshare is not supported on remote clients")
}

func (ic ContainerEngine) Version(ctx context.Context) (*entities.SystemVersionReport, error) {
	return system.Version(ic.ClientCtx, nil)
}
