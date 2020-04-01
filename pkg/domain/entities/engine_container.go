package entities

import (
	"context"

	"github.com/containers/libpod/libpod/define"
)

type ContainerEngine interface {
	ContainerCommit(ctx context.Context, nameOrId string, options CommitOptions) (*CommitReport, error)
	ContainerExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	ContainerInspect(ctx context.Context, namesOrIds []string, options InspectOptions) ([]*ContainerInspectReport, error)
	ContainerKill(ctx context.Context, namesOrIds []string, options KillOptions) ([]*KillReport, error)
	ContainerPause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerRestart(ctx context.Context, namesOrIds []string, options RestartOptions) ([]*RestartReport, error)
	ContainerRm(ctx context.Context, namesOrIds []string, options RmOptions) ([]*RmReport, error)
	ContainerStop(ctx context.Context, namesOrIds []string, options StopOptions) ([]*StopReport, error)
	ContainerTop(ctx context.Context, options TopOptions) (*StringSliceReport, error)
	ContainerUnpause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
	HealthCheckRun(ctx context.Context, nameOrId string, options HealthCheckOptions) (*define.HealthCheckResults, error)
	PodCreate(ctx context.Context, opts PodCreateOptions) (*PodCreateReport, error)
	PodExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	PodKill(ctx context.Context, namesOrIds []string, options PodKillOptions) ([]*PodKillReport, error)
	PodPause(ctx context.Context, namesOrIds []string, options PodPauseOptions) ([]*PodPauseReport, error)
	PodPs(ctx context.Context, options PodPSOptions) ([]*ListPodsReport, error)
	PodRestart(ctx context.Context, namesOrIds []string, options PodRestartOptions) ([]*PodRestartReport, error)
	PodRm(ctx context.Context, namesOrIds []string, options PodRmOptions) ([]*PodRmReport, error)
	PodStart(ctx context.Context, namesOrIds []string, options PodStartOptions) ([]*PodStartReport, error)
	PodStop(ctx context.Context, namesOrIds []string, options PodStopOptions) ([]*PodStopReport, error)
	PodTop(ctx context.Context, options PodTopOptions) (*StringSliceReport, error)
	PodUnpause(ctx context.Context, namesOrIds []string, options PodunpauseOptions) ([]*PodUnpauseReport, error)
	VolumeCreate(ctx context.Context, opts VolumeCreateOptions) (*IdOrNameResponse, error)
	VolumeInspect(ctx context.Context, namesOrIds []string, opts VolumeInspectOptions) ([]*VolumeInspectReport, error)
	VolumeList(ctx context.Context, opts VolumeListOptions) ([]*VolumeListReport, error)
	VolumePrune(ctx context.Context, opts VolumePruneOptions) ([]*VolumePruneReport, error)
	VolumeRm(ctx context.Context, namesOrIds []string, opts VolumeRmOptions) ([]*VolumeRmReport, error)
}
