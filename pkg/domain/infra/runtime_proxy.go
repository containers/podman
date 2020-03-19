// +build ABISupport

package infra

import (
	"context"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	flag "github.com/spf13/pflag"
)

// ContainerEngine Proxy will be EOL'ed after podmanV2 is separated from libpod repo

type runtime struct {
	entities.ContainerEngine
	Libpod *libpod.Runtime
}

func NewLibpodRuntime(flags flag.FlagSet, opts entities.EngineFlags) (entities.ContainerEngine, error) {
	r, err := GetRuntime(context.Background(), flags, opts)
	if err != nil {
		return nil, err
	}
	return &runtime{Libpod: r}, nil
}

func (r *runtime) ShutdownRuntime(force bool) error {
	return r.Libpod.Shutdown(force)
}

func (r *runtime) ContainerDelete(ctx context.Context, opts entities.ContainerDeleteOptions) (*entities.ContainerDeleteReport, error) {
	panic("implement me")
}

func (r *runtime) ContainerPrune(ctx context.Context) (*entities.ContainerPruneReport, error) {
	panic("implement me")
}

func (r *runtime) PodDelete(ctx context.Context, opts entities.PodPruneOptions) (*entities.PodDeleteReport, error) {
	panic("implement me")
}

func (r *runtime) PodPrune(ctx context.Context) (*entities.PodPruneReport, error) {
	panic("implement me")
}

func (r *runtime) VolumeDelete(ctx context.Context, opts entities.VolumeDeleteOptions) (*entities.VolumeDeleteReport, error) {
	panic("implement me")
}

func (r *runtime) VolumePrune(ctx context.Context) (*entities.VolumePruneReport, error) {
	panic("implement me")
}
