package entities

import (
	"context"
)

type ImageEngine interface {
	Delete(ctx context.Context, nameOrId string, opts ImageDeleteOptions) (*ImageDeleteReport, error)
	Exists(ctx context.Context, nameOrId string) (*BoolReport, error)
	History(ctx context.Context, nameOrId string, opts ImageHistoryOptions) (*ImageHistoryReport, error)
	List(ctx context.Context, opts ImageListOptions) ([]*ImageSummary, error)
	Prune(ctx context.Context, opts ImagePruneOptions) (*ImagePruneReport, error)
}
