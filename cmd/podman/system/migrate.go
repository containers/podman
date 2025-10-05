//go:build !remote

package system

import (
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
}

func migrate(_ *cobra.Command, _ []string) error {
	return registry.ContainerEngine().Migrate(registry.Context(), migrateOptions)
}
