package entities

import (
	"context"
)

type ContainerEngine interface {
	ContainerRuntime
	PodRuntime
	VolumeRuntime
}

type ContainerRuntime interface {
	ContainerDelete(ctx context.Context, opts ContainerDeleteOptions) (*ContainerDeleteReport, error)
	ContainerPrune(ctx context.Context) (*ContainerPruneReport, error)
}

type PodRuntime interface {
	PodDelete(ctx context.Context, opts PodPruneOptions) (*PodDeleteReport, error)
	PodPrune(ctx context.Context) (*PodPruneReport, error)
}

type VolumeRuntime interface {
	VolumeDelete(ctx context.Context, opts VolumeDeleteOptions) (*VolumeDeleteReport, error)
	VolumePrune(ctx context.Context) (*VolumePruneReport, error)
}
