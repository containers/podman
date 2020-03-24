package entities

import (
	"context"
)

type ContainerEngine interface {
	ContainerPrune(ctx context.Context) (*ContainerPruneReport, error)
	ContainerExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	ContainerKill(ctx context.Context, namesOrIds []string, options KillOptions) ([]*KillReport, error)
	ContainerPause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerRestart(ctx context.Context, namesOrIds []string, options RestartOptions) ([]*RestartReport, error)
	ContainerRm(ctx context.Context, namesOrIds []string, options RmOptions) ([]*RmReport, error)
	ContainerUnpause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerStop(ctx context.Context, namesOrIds []string, options StopOptions) ([]*StopReport, error)
	ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
	PodDelete(ctx context.Context, opts PodPruneOptions) (*PodDeleteReport, error)
	PodExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	PodPrune(ctx context.Context) (*PodPruneReport, error)
	VolumeCreate(ctx context.Context, opts VolumeCreateOptions) (*IdOrNameResponse, error)
	VolumeDelete(ctx context.Context, opts VolumeDeleteOptions) (*VolumeDeleteReport, error)
	VolumePrune(ctx context.Context) (*VolumePruneReport, error)
}
