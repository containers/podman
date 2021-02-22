package entities

import (
	"context"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/pkg/domain/entities/reports"
)

type ImageEngine interface {
	Build(ctx context.Context, containerFiles []string, opts BuildOptions) (*BuildReport, error)
	Config(ctx context.Context) (*config.Config, error)
	Diff(ctx context.Context, nameOrID string, options DiffOptions) (*DiffReport, error)
	Exists(ctx context.Context, nameOrID string) (*BoolReport, error)
	History(ctx context.Context, nameOrID string, opts ImageHistoryOptions) (*ImageHistoryReport, error)
	Import(ctx context.Context, opts ImageImportOptions) (*ImageImportReport, error)
	Inspect(ctx context.Context, namesOrIDs []string, opts InspectOptions) ([]*ImageInspectReport, []error, error)
	List(ctx context.Context, opts ImageListOptions) ([]*ImageSummary, error)
	Load(ctx context.Context, opts ImageLoadOptions) (*ImageLoadReport, error)
	Mount(ctx context.Context, images []string, options ImageMountOptions) ([]*ImageMountReport, error)
	Prune(ctx context.Context, opts ImagePruneOptions) ([]*reports.PruneReport, error)
	Pull(ctx context.Context, rawImage string, opts ImagePullOptions) (*ImagePullReport, error)
	Push(ctx context.Context, source string, destination string, opts ImagePushOptions) error
	Remove(ctx context.Context, images []string, opts ImageRemoveOptions) (*ImageRemoveReport, []error)
	Save(ctx context.Context, nameOrID string, tags []string, options ImageSaveOptions) error
	Search(ctx context.Context, term string, opts ImageSearchOptions) ([]ImageSearchReport, error)
	SetTrust(ctx context.Context, args []string, options SetTrustOptions) error
	ShowTrust(ctx context.Context, args []string, options ShowTrustOptions) (*ShowTrustReport, error)
	Shutdown(ctx context.Context)
	Tag(ctx context.Context, nameOrID string, tags []string, options ImageTagOptions) error
	Tree(ctx context.Context, nameOrID string, options ImageTreeOptions) (*ImageTreeReport, error)
	Unmount(ctx context.Context, images []string, options ImageUnmountOptions) ([]*ImageUnmountReport, error)
	Untag(ctx context.Context, nameOrID string, tags []string, options ImageUntagOptions) error
	ManifestCreate(ctx context.Context, names, images []string, opts ManifestCreateOptions) (string, error)
	ManifestExists(ctx context.Context, name string) (*BoolReport, error)
	ManifestInspect(ctx context.Context, name string) ([]byte, error)
	ManifestAdd(ctx context.Context, opts ManifestAddOptions) (string, error)
	ManifestAnnotate(ctx context.Context, names []string, opts ManifestAnnotateOptions) (string, error)
	ManifestRemove(ctx context.Context, names []string) (string, error)
	ManifestPush(ctx context.Context, name, destination string, imagePushOpts ImagePushOptions) (string, error)
	Sign(ctx context.Context, names []string, options SignOptions) (*SignReport, error)
}
