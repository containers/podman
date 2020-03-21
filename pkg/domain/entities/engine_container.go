package entities

import (
	"context"
)

type ContainerEngine interface {
	ContainerDelete(ctx context.Context, opts ContainerDeleteOptions) (*ContainerDeleteReport, error)
	ContainerPrune(ctx context.Context) (*ContainerPruneReport, error)
	ContainerExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
	PodDelete(ctx context.Context, opts PodPruneOptions) (*PodDeleteReport, error)
	PodExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	PodPrune(ctx context.Context) (*PodPruneReport, error)
	VolumeCreate(ctx context.Context, opts VolumeCreateOptions) (*IdOrNameResponse, error)
	VolumeDelete(ctx context.Context, opts VolumeDeleteOptions) (*VolumeDeleteReport, error)
	VolumePrune(ctx context.Context) (*VolumePruneReport, error)
}
