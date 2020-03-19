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
