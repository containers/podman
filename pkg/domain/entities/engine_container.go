package entities

import (
	"context"
	"io"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities/reports"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/spf13/cobra"
)

type ContainerCopyFunc func() error

type ContainerEngine interface {
	AutoUpdate(ctx context.Context, options AutoUpdateOptions) (*AutoUpdateReport, []error)
	Config(ctx context.Context) (*config.Config, error)
	ContainerAttach(ctx context.Context, nameOrID string, options AttachOptions) error
	ContainerCheckpoint(ctx context.Context, namesOrIds []string, options CheckpointOptions) ([]*CheckpointReport, error)
	ContainerCleanup(ctx context.Context, namesOrIds []string, options ContainerCleanupOptions) ([]*ContainerCleanupReport, error)
	ContainerCommit(ctx context.Context, nameOrID string, options CommitOptions) (*CommitReport, error)
	ContainerCopyFromArchive(ctx context.Context, nameOrID string, path string, reader io.Reader) (ContainerCopyFunc, error)
	ContainerCopyToArchive(ctx context.Context, nameOrID string, path string, writer io.Writer) (ContainerCopyFunc, error)
	ContainerCreate(ctx context.Context, s *specgen.SpecGenerator) (*ContainerCreateReport, error)
	ContainerDiff(ctx context.Context, nameOrID string, options DiffOptions) (*DiffReport, error)
	ContainerExec(ctx context.Context, nameOrID string, options ExecOptions, streams define.AttachStreams) (int, error)
	ContainerExecDetached(ctx context.Context, nameOrID string, options ExecOptions) (string, error)
	ContainerExists(ctx context.Context, nameOrID string, options ContainerExistsOptions) (*BoolReport, error)
	ContainerExport(ctx context.Context, nameOrID string, options ContainerExportOptions) error
	ContainerInit(ctx context.Context, namesOrIds []string, options ContainerInitOptions) ([]*ContainerInitReport, error)
	ContainerInspect(ctx context.Context, namesOrIds []string, options InspectOptions) ([]*ContainerInspectReport, []error, error)
	ContainerKill(ctx context.Context, namesOrIds []string, options KillOptions) ([]*KillReport, error)
	ContainerList(ctx context.Context, options ContainerListOptions) ([]ListContainer, error)
	ContainerLogs(ctx context.Context, containers []string, options ContainerLogsOptions) error
	ContainerMount(ctx context.Context, nameOrIDs []string, options ContainerMountOptions) ([]*ContainerMountReport, error)
	ContainerPause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerPort(ctx context.Context, nameOrID string, options ContainerPortOptions) ([]*ContainerPortReport, error)
	ContainerPrune(ctx context.Context, options ContainerPruneOptions) ([]*reports.PruneReport, error)
	ContainerRename(ctr context.Context, nameOrID string, options ContainerRenameOptions) error
	ContainerRestart(ctx context.Context, namesOrIds []string, options RestartOptions) ([]*RestartReport, error)
	ContainerRestore(ctx context.Context, namesOrIds []string, options RestoreOptions) ([]*RestoreReport, error)
	ContainerRm(ctx context.Context, namesOrIds []string, options RmOptions) ([]*RmReport, error)
	ContainerRun(ctx context.Context, opts ContainerRunOptions) (*ContainerRunReport, error)
	ContainerRunlabel(ctx context.Context, label string, image string, args []string, opts ContainerRunlabelOptions) error
	ContainerStart(ctx context.Context, namesOrIds []string, options ContainerStartOptions) ([]*ContainerStartReport, error)
	ContainerStat(ctx context.Context, nameOrDir string, path string) (*ContainerStatReport, error)
	ContainerStats(ctx context.Context, namesOrIds []string, options ContainerStatsOptions) (chan ContainerStatsReport, error)
	ContainerStop(ctx context.Context, namesOrIds []string, options StopOptions) ([]*StopReport, error)
	ContainerTop(ctx context.Context, options TopOptions) (*StringSliceReport, error)
	ContainerUnmount(ctx context.Context, nameOrIDs []string, options ContainerUnmountOptions) ([]*ContainerUnmountReport, error)
	ContainerUnpause(ctx context.Context, namesOrIds []string, options PauseUnPauseOptions) ([]*PauseUnpauseReport, error)
	ContainerWait(ctx context.Context, namesOrIds []string, options WaitOptions) ([]WaitReport, error)
	Events(ctx context.Context, opts EventsOptions) error
	GenerateSystemd(ctx context.Context, nameOrID string, opts GenerateSystemdOptions) (*GenerateSystemdReport, error)
	GenerateKube(ctx context.Context, nameOrIDs []string, opts GenerateKubeOptions) (*GenerateKubeReport, error)
	SystemPrune(ctx context.Context, options SystemPruneOptions) (*SystemPruneReport, error)
	HealthCheckRun(ctx context.Context, nameOrID string, options HealthCheckOptions) (*define.HealthCheckResults, error)
	Info(ctx context.Context) (*define.Info, error)
	NetworkConnect(ctx context.Context, networkname string, options NetworkConnectOptions) error
	NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (*NetworkCreateReport, error)
	NetworkDisconnect(ctx context.Context, networkname string, options NetworkDisconnectOptions) error
	NetworkExists(ctx context.Context, networkname string) (*BoolReport, error)
	NetworkInspect(ctx context.Context, namesOrIds []string, options InspectOptions) ([]NetworkInspectReport, []error, error)
	NetworkList(ctx context.Context, options NetworkListOptions) ([]*NetworkListReport, error)
	NetworkPrune(ctx context.Context, options NetworkPruneOptions) ([]*NetworkPruneReport, error)
	NetworkReload(ctx context.Context, names []string, options NetworkReloadOptions) ([]*NetworkReloadReport, error)
	NetworkRm(ctx context.Context, namesOrIds []string, options NetworkRmOptions) ([]*NetworkRmReport, error)
	PlayKube(ctx context.Context, path string, opts PlayKubeOptions) (*PlayKubeReport, error)
	PodCreate(ctx context.Context, opts PodCreateOptions) (*PodCreateReport, error)
	PodExists(ctx context.Context, nameOrID string) (*BoolReport, error)
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
	SecretCreate(ctx context.Context, name string, reader io.Reader, options SecretCreateOptions) (*SecretCreateReport, error)
	SecretInspect(ctx context.Context, nameOrIDs []string) ([]*SecretInfoReport, []error, error)
	SecretList(ctx context.Context) ([]*SecretInfoReport, error)
	SecretRm(ctx context.Context, nameOrID []string, opts SecretRmOptions) ([]*SecretRmReport, error)
	Shutdown(ctx context.Context)
	SystemDf(ctx context.Context, options SystemDfOptions) (*SystemDfReport, error)
	Unshare(ctx context.Context, args []string, options SystemUnshareOptions) error
	Version(ctx context.Context) (*SystemVersionReport, error)
	VolumeCreate(ctx context.Context, opts VolumeCreateOptions) (*IDOrNameResponse, error)
	VolumeExists(ctx context.Context, namesOrID string) (*BoolReport, error)
	VolumeInspect(ctx context.Context, namesOrIds []string, opts InspectOptions) ([]*VolumeInspectReport, []error, error)
	VolumeList(ctx context.Context, opts VolumeListOptions) ([]*VolumeListReport, error)
	VolumePrune(ctx context.Context, options VolumePruneOptions) ([]*reports.PruneReport, error)
	VolumeRm(ctx context.Context, namesOrIds []string, opts VolumeRmOptions) ([]*VolumeRmReport, error)
}
