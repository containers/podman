package entities

import (
	"context"

	"github.com/spf13/pflag"
)

type SystemEngine interface {
	Renumber(ctx context.Context, flags *pflag.FlagSet, config *PodmanConfig) error
	Migrate(ctx context.Context, flags *pflag.FlagSet, config *PodmanConfig, options SystemMigrateOptions) error
	Reset(ctx context.Context) error
	Shutdown(ctx context.Context)
}
