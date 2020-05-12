package entities

import (
	"context"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/spf13/cobra"
)

type ContainerEngine interface {
	AutoUpdate(ctx context.Context, options AutoUpdateOptions) (*AutoUpdateReport, []error)
	Config(ctx context.Context) (*config.Config, error)
	ContainerAttach(ctx context.Context, nameOrId string, options AttachOptions) error
	ContainerCheckpoint(ctx context.Context, namesOrIds []string, options CheckpointOptions) ([]*CheckpointReport, error)
	ContainerCleanup(ctx context.Context, namesOrIds []string, options ContainerCleanupOptions) ([]*ContainerCleanupReport, error)
	ContainerCommit(ctx context.Context, nameOrId string, options CommitOptions) (*CommitReport, error)
	ContainerCp(ctx context.Context, source, dest string, options ContainerCpOptions) (*ContainerCpReport, error)
	ContainerCreate(ctx context.Context, s *specgen.SpecGenerator) (*ContainerCreateReport, error)
	ContainerDiff(ctx context.Context, nameOrId string, options DiffOptions) (*DiffReport, error)
	ContainerExec(ctx context.Context, nameOrId string, options ExecOptions) (int, error)
	ContainerExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	ContainerExport(ctx context.Context, nameOrId string, options ContainerExportOptions) error
	ContainerInit(ctx context.Context, namesOrIds []string, options ContainerInitOptions) ([]*ContainerInitReport, error)
	ContainerInspect(ctx context.Context, namesOrIds []string, options InspectOptions) ([]*ContainerInspectReport, error)
	ContainerKill(ctx context.Context, namesOrIds []string, options KillOptions) ([]*KillReport, error)
	ContainerList(ctx context.Context, options ContainerListOptions) ([]ListContainer, error)
	ContainerLogs(ctx context.Context, containers []string, options ContainerLogsOptions) error
	ContainerMount(ctx context.Context, nameOrIds []string, options ContainerMountOptions) ([]*ContainerMountReport, error)
	ContainerPause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerPort(ctx context.Context, nameOrId string, options ContainerPortOptions) ([]*ContainerPortReport, error)
	ContainerPrune(ctx context.Context, options ContainerPruneOptions) (*ContainerPruneReport, error)
	ContainerRestart(ctx context.Context, namesOrIds []string, options RestartOptions) ([]*RestartReport, error)
	ContainerRestore(ctx context.Context, namesOrIds []string, options RestoreOptions) ([]*RestoreReport, error)
	ContainerRm(ctx context.Context, namesOrIds []string, options RmOptions) ([]*RmReport, error)
	ContainerRun(ctx context.Context, opts ContainerRunOptions) (*ContainerRunReport, error)
	ContainerRunlabel(ctx context.Context, label string, image string, args []string, opts ContainerRunlabelOptions) error
	ContainerStart(ctx context.Context, namesOrIds []string, options ContainerStartOptions) ([]*ContainerStartReport, error)
	ContainerStats(ctx context.Context, namesOrIds []string, options ContainerStatsOptions) error
	ContainerStop(ctx context.Context, namesOrIds []string, options StopOptions) ([]*StopReport, error)
	ContainerTop(ctx context.Context, options TopOptions) (*StringSliceReport, error)
	ContainerUnmount(ctx context.Context, nameOrIds []string, options ContainerUnmountOptions) ([]*ContainerUnmountReport, error)
	ContainerUnpause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
	Events(ctx context.Context, opts EventsOptions) error
	GenerateSystemd(ctx context.Context, nameOrID string, opts GenerateSystemdOptions) (*GenerateSystemdReport, error)
	GenerateKube(ctx context.Context, nameOrID string, opts GenerateKubeOptions) (*GenerateKubeReport, error)
	SystemPrune(ctx context.Context, options SystemPruneOptions) (*SystemPruneReport, error)
	HealthCheckRun(ctx context.Context, nameOrId string, options HealthCheckOptions) (*define.HealthCheckResults, error)
	Info(ctx context.Context) (*define.Info, error)
	NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (*NetworkCreateReport, error)
	NetworkInspect(ctx context.Context, namesOrIds []string, options NetworkInspectOptions) ([]NetworkInspectReport, error)
	NetworkList(ctx context.Context, options NetworkListOptions) ([]*NetworkListReport, error)
	NetworkRm(ctx context.Context, namesOrIds []string, options NetworkRmOptions) ([]*NetworkRmReport, error)
	PlayKube(ctx context.Context, path string, opts PlayKubeOptions) (*PlayKubeReport, error)
	PodCreate(ctx context.Context, opts PodCreateOptions) (*PodCreateReport, error)
	PodExists(ctx context.Context, nameOrId string) (*BoolReport, error)
	PodInspect(ctx context.Context, options PodInspectOptions) (*PodInspectReport, error)
	PodKill(ctx context.Context, namesOrIds []string, options PodKillOptions) ([]*PodKillReport, error)
	PodPause(ctx context.Context, namesOrIds []string, options PodPauseOptions) ([]*PodPauseReport, error)
	PodPrune(ctx context.Context, options PodPruneOptions) ([]*PodPruneReport, error)
	PodPs(ctx context.Context, options PodPSOptions) ([]*ListPodsReport, error)
	PodRestart(ctx context.Context, namesOrIds []string, options PodRestartOptions) ([]*PodRestartReport, error)
	PodRm(ctx context.Context, namesOrIds []string, options PodRmOptions) ([]*PodRmReport, error)
	PodStart(ctx context.Context, namesOrIds []string, options PodStartOptions) ([]*PodStartReport, error)
	PodStats(ctx context.Context, namesOrIds []string, options PodStatsOptions) ([]*PodStatsReport, error)
	PodStop(ctx context.Context, namesOrIds []string, options PodStopOptions) ([]*PodStopReport, error)
	PodTop(ctx context.Context, options PodTopOptions) (*StringSliceReport, error)
	PodUnpause(ctx context.Context, namesOrIds []string, options PodunpauseOptions) ([]*PodUnpauseReport, error)
	SetupRootless(ctx context.Context, cmd *cobra.Command) error
	Shutdown(ctx context.Context)
	SystemDf(ctx context.Context, options SystemDfOptions) (*SystemDfReport, error)
	Unshare(ctx context.Context, args []string) error
	VarlinkService(ctx context.Context, opts ServiceOptions) error
	Version(ctx context.Context) (*SystemVersionReport, error)
	VolumeCreate(ctx context.Context, opts VolumeCreateOptions) (*IdOrNameResponse, error)
	VolumeInspect(ctx context.Context, namesOrIds []string, opts VolumeInspectOptions) ([]*VolumeInspectReport, error)
	VolumeList(ctx context.Context, opts VolumeListOptions) ([]*VolumeListReport, error)
	VolumePrune(ctx context.Context, opts VolumePruneOptions) ([]*VolumePruneReport, error)
	VolumeRm(ctx context.Context, namesOrIds []string, opts VolumeRmOptions) ([]*VolumeRmReport, error)
}
