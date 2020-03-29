package entities

import (
	"context"
)

type ContainerEngine interface {
	ContainerCommit(ctx context.Context, nameOrId string, options CommitOptions) (*CommitReport, error)
	ContainerExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	ContainerInspect(ctx context.Context, namesOrIds []string, options ContainerInspectOptions) ([]*ContainerInspectReport, error)
	ContainerKill(ctx context.Context, namesOrIds []string, options KillOptions) ([]*KillReport, error)
	ContainerPause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerRestart(ctx context.Context, namesOrIds []string, options RestartOptions) ([]*RestartReport, error)
	ContainerRm(ctx context.Context, namesOrIds []string, options RmOptions) ([]*RmReport, error)
	ContainerUnpause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerStop(ctx context.Context, namesOrIds []string, options StopOptions) ([]*StopReport, error)
	ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
	ContainerTop(ctx context.Context, options TopOptions) (*StringSliceReport, error)
	PodCreate(ctx context.Context, opts PodCreateOptions) (*PodCreateReport, error)
	PodExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	PodKill(ctx context.Context, namesOrIds []string, options PodKillOptions) ([]*PodKillReport, error)
	PodPause(ctx context.Context, namesOrIds []string, options PodPauseOptions) ([]*PodPauseReport, error)
	PodRestart(ctx context.Context, namesOrIds []string, options PodRestartOptions) ([]*PodRestartReport, error)
	PodStart(ctx context.Context, namesOrIds []string, options PodStartOptions) ([]*PodStartReport, error)
	PodStop(ctx context.Context, namesOrIds []string, options PodStopOptions) ([]*PodStopReport, error)
	PodRm(ctx context.Context, namesOrIds []string, options PodRmOptions) ([]*PodRmReport, error)
	PodUnpause(ctx context.Context, namesOrIds []string, options PodunpauseOptions) ([]*PodUnpauseReport, error)

	VolumeCreate(ctx context.Context, opts VolumeCreateOptions) (*IdOrNameResponse, error)
	VolumeInspect(ctx context.Context, namesOrIds []string, opts VolumeInspectOptions) ([]*VolumeInspectReport, error)
	VolumeRm(ctx context.Context, namesOrIds []string, opts VolumeRmOptions) ([]*VolumeRmReport, error)
	VolumePrune(ctx context.Context, opts VolumePruneOptions) ([]*VolumePruneReport, error)
	VolumeList(ctx context.Context, opts VolumeListOptions) ([]*VolumeListReport, error)
}
