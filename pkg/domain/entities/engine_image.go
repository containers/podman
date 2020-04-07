package entities

import (
	"context"
)

type ImageEngine interface {
	Delete(ctx context.Context, nameOrId []string, opts ImageDeleteOptions) (*ImageDeleteReport, error)
	Diff(ctx context.Context, nameOrId string, options DiffOptions) (*DiffReport, error)
	Exists(ctx context.Context, nameOrId string) (*BoolReport, error)
	History(ctx context.Context, nameOrId string, opts ImageHistoryOptions) (*ImageHistoryReport, error)
	Import(ctx context.Context, opts ImageImportOptions) (*ImageImportReport, error)
	Inspect(ctx context.Context, names []string, opts InspectOptions) (*ImageInspectReport, error)
	List(ctx context.Context, opts ImageListOptions) ([]*ImageSummary, error)
	Load(ctx context.Context, opts ImageLoadOptions) (*ImageLoadReport, error)
	Prune(ctx context.Context, opts ImagePruneOptions) (*ImagePruneReport, error)
	Pull(ctx context.Context, rawImage string, opts ImagePullOptions) (*ImagePullReport, error)
	Push(ctx context.Context, source string, destination string, opts ImagePushOptions) error
	Save(ctx context.Context, nameOrId string, tags []string, options ImageSaveOptions) error
	Tag(ctx context.Context, nameOrId string, tags []string, options ImageTagOptions) error
	Untag(ctx context.Context, nameOrId string, tags []string, options ImageUntagOptions) error
}
