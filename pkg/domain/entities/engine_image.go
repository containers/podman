package entities

import (
	"context"
)

type ImageEngine interface {
	Delete(ctx context.Context, nameOrId []string, opts ImageDeleteOptions) (*ImageDeleteReport, error)
	Exists(ctx context.Context, nameOrId string) (*BoolReport, error)
	History(ctx context.Context, nameOrId string, opts ImageHistoryOptions) (*ImageHistoryReport, error)
	Inspect(ctx context.Context, names []string, opts InspectOptions) (*ImageInspectReport, error)
	List(ctx context.Context, opts ImageListOptions) ([]*ImageSummary, error)
	Prune(ctx context.Context, opts ImagePruneOptions) (*ImagePruneReport, error)
	Pull(ctx context.Context, rawImage string, opts ImagePullOptions) (*ImagePullReport, error)
	Tag(ctx context.Context, nameOrId string, tags []string, options ImageTagOptions) error
	Untag(ctx context.Context, nameOrId string, tags []string, options ImageUntagOptions) error
	Load(ctx context.Context, opts ImageLoadOptions) (*ImageLoadReport, error)
	Import(ctx context.Context, opts ImageImportOptions) (*ImageImportReport, error)
	Push(ctx context.Context, source string, destination string, opts ImagePushOptions) error
}
