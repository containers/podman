package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/domain/entities"
)

// Image-related runtime using an ssh-tunnel to utilize Podman service
type ImageEngine struct {
	ClientCxt context.Context
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type ContainerEngine struct {
	ClientCxt context.Context
}

func (r *ContainerEngine) Shutdown(force bool) error {
	return nil
}

func (r *ContainerEngine) ContainerDelete(ctx context.Context, opts entities.ContainerDeleteOptions) (*entities.ContainerDeleteReport, error) {
	panic("implement me")
}

func (r *ContainerEngine) ContainerPrune(ctx context.Context) (*entities.ContainerPruneReport, error) {
	panic("implement me")
}

func (r *ContainerEngine) PodDelete(ctx context.Context, opts entities.PodPruneOptions) (*entities.PodDeleteReport, error) {
	panic("implement me")
}

func (r *ContainerEngine) PodPrune(ctx context.Context) (*entities.PodPruneReport, error) {
	panic("implement me")
}

func (r *ContainerEngine) VolumeDelete(ctx context.Context, opts entities.VolumeDeleteOptions) (*entities.VolumeDeleteReport, error) {
	panic("implement me")
}

func (r *ContainerEngine) VolumePrune(ctx context.Context) (*entities.VolumePruneReport, error) {
	panic("implement me")
}
