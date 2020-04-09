package entities

import (
	"context"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/specgen"
)

type ContainerEngine interface {
	ContainerAttach(ctx context.Context, nameOrId string, options AttachOptions) error
	ContainerCheckpoint(ctx context.Context, namesOrIds []string, options CheckpointOptions) ([]*CheckpointReport, error)
	ContainerCleanup(ctx context.Context, namesOrIds []string, options ContainerCleanupOptions) ([]*ContainerCleanupReport, error)
	ContainerCommit(ctx context.Context, nameOrId string, options CommitOptions) (*CommitReport, error)
	ContainerCreate(ctx context.Context, s *specgen.SpecGenerator) (*ContainerCreateReport, error)
	ContainerDiff(ctx context.Context, nameOrId string, options DiffOptions) (*DiffReport, error)
	ContainerExec(ctx context.Context, nameOrId string, options ExecOptions) (int, error)
	ContainerExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	ContainerExport(ctx context.Context, nameOrId string, options ContainerExportOptions) error
	ContainerInspect(ctx context.Context, namesOrIds []string, options InspectOptions) ([]*ContainerInspectReport, error)
	ContainerKill(ctx context.Context, namesOrIds []string, options KillOptions) ([]*KillReport, error)
	ContainerList(ctx context.Context, options ContainerListOptions) ([]ListContainer, error)
	ContainerPause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerLogs(ctx context.Context, containers []string, options ContainerLogsOptions) error
	ContainerRestart(ctx context.Context, namesOrIds []string, options RestartOptions) ([]*RestartReport, error)
	ContainerRestore(ctx context.Context, namesOrIds []string, options RestoreOptions) ([]*RestoreReport, error)
	ContainerRm(ctx context.Context, namesOrIds []string, options RmOptions) ([]*RmReport, error)
	ContainerRun(ctx context.Context, opts ContainerRunOptions) (*ContainerRunReport, error)
	ContainerStart(ctx context.Context, namesOrIds []string, options ContainerStartOptions) ([]*ContainerStartReport, error)
	ContainerStop(ctx context.Context, namesOrIds []string, options StopOptions) ([]*StopReport, error)
	ContainerTop(ctx context.Context, options TopOptions) (*StringSliceReport, error)
	ContainerUnpause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
	HealthCheckRun(ctx context.Context, nameOrId string, options HealthCheckOptions) (*define.HealthCheckResults, error)
	Info(ctx context.Context) (*define.Info, error)
	PodCreate(ctx context.Context, opts PodCreateOptions) (*PodCreateReport, error)
	PodExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	PodInspect(ctx context.Context, options PodInspectOptions) (*PodInspectReport, error)
	PodKill(ctx context.Context, namesOrIds []string, options PodKillOptions) ([]*PodKillReport, error)
	PodPause(ctx context.Context, namesOrIds []string, options PodPauseOptions) ([]*PodPauseReport, error)
	PodPs(ctx context.Context, options PodPSOptions) ([]*ListPodsReport, error)
	PodRestart(ctx context.Context, namesOrIds []string, options PodRestartOptions) ([]*PodRestartReport, error)
	PodRm(ctx context.Context, namesOrIds []string, options PodRmOptions) ([]*PodRmReport, error)
	PodStart(ctx context.Context, namesOrIds []string, options PodStartOptions) ([]*PodStartReport, error)
	PodStop(ctx context.Context, namesOrIds []string, options PodStopOptions) ([]*PodStopReport, error)
	PodTop(ctx context.Context, options PodTopOptions) (*StringSliceReport, error)
	PodUnpause(ctx context.Context, namesOrIds []string, options PodunpauseOptions) ([]*PodUnpauseReport, error)
	RestService(ctx context.Context, opts ServiceOptions) error
	VarlinkService(ctx context.Context, opts ServiceOptions) error
	VolumeCreate(ctx context.Context, opts VolumeCreateOptions) (*IdOrNameResponse, error)
	VolumeInspect(ctx context.Context, namesOrIds []string, opts VolumeInspectOptions) ([]*VolumeInspectReport, error)
	VolumeList(ctx context.Context, opts VolumeListOptions) ([]*VolumeListReport, error)
	VolumePrune(ctx context.Context, opts VolumePruneOptions) ([]*VolumePruneReport, error)
	VolumeRm(ctx context.Context, namesOrIds []string, opts VolumeRmOptions) ([]*VolumeRmReport, error)
}
