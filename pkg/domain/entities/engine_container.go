package entities

import (
	"context"
)

type ContainerEngine interface {
	ContainerDelete(ctx context.Context, opts ContainerDeleteOptions) (*ContainerDeleteReport, error)
	ContainerPrune(ctx context.Context) (*ContainerPruneReport, error)
	ContainerExists(ctx context.Context, nameOrId string) (bool, error)
	ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
	PodDelete(ctx context.Context, opts PodPruneOptions) (*PodDeleteReport, error)
	PodPrune(ctx context.Context) (*PodPruneReport, error)
	VolumeDelete(ctx context.Context, opts VolumeDeleteOptions) (*VolumeDeleteReport, error)
	VolumePrune(ctx context.Context) (*VolumePruneReport, error)
}
