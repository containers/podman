//go:build !remote

package system

import (
	"os"

	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
)

var (
	migrateDescription = `
        podman system migrate

        Migrate existing containers to a new version of Podman.
`

	migrateCommand = &cobra.Command{
		Annotations: map[string]string{
			registry.EngineMode:    registry.ABIMode,
			registry.NoMoveProcess: registry.NoMoveProcess,
		},
		Use:               "migrate [options]",
		Args:              validate.NoArgs,
		Short:             "Migrate containers",
		Long:              migrateDescription,
		RunE:              migrate,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	migrateOptions entities.SystemMigrateOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: migrateCommand,
		Parent:  systemCmd,
	})

	flags := migrateCommand.Flags()

	newRuntimeFlagName := "new-runtime"
	flags.StringVar(&migrateOptions.NewRuntime, newRuntimeFlagName, "", "Specify a new runtime for all containers")
	_ = migrateCommand.RegisterFlagCompletionFunc(newRuntimeFlagName, completion.AutocompleteNone)

	flags.BoolVar(&migrateOptions.MigrateDB, "migrate-db", false, "Migrate database from BoltDB to SQLite")
}

func migrate(_ *cobra.Command, _ []string) error {
	// HACK: do not warn about a database migration being needed, when we are about to migrate the database.
	os.Setenv("SUPPRESS_BOLTDB_WARNING", "1")

	return registry.ContainerEngine().Migrate(registry.Context(), migrateOptions)
}
