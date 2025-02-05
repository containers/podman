package tunnel

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/domain/entities"
)

// TODO For now, no remote support has been added. We need the API to firm up first.

func ArtifactAdd(ctx context.Context, path, name string, opts entities.ArtifactAddOptions) error {
	return fmt.Errorf("not implemented")
}

func (ir *ImageEngine) ArtifactInspect(ctx context.Context, name string, opts entities.ArtifactInspectOptions) (*entities.ArtifactInspectReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ir *ImageEngine) ArtifactList(ctx context.Context, opts entities.ArtifactListOptions) ([]*entities.ArtifactListReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ir *ImageEngine) ArtifactPull(ctx context.Context, name string, opts entities.ArtifactPullOptions) (*entities.ArtifactPullReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ir *ImageEngine) ArtifactRm(ctx context.Context, name string, opts entities.ArtifactRemoveOptions) (*entities.ArtifactRemoveReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ir *ImageEngine) ArtifactPush(ctx context.Context, name string, opts entities.ArtifactPushOptions) (*entities.ArtifactPushReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ir *ImageEngine) ArtifactAdd(ctx context.Context, name string, paths []string, opts *entities.ArtifactAddOptions) (*entities.ArtifactAddReport, error) {
	return nil, fmt.Errorf("not implemented")
}
