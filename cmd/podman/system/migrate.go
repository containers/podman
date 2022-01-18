// +build !remote

package system

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra"
	"github.com/spf13/cobra"
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
		Run:               migrate,
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

func migrate(cmd *cobra.Command, args []string) {
	// Shutdown all running engines, `renumber` will hijack repository
	registry.ContainerEngine().Shutdown(registry.Context())
	registry.ImageEngine().Shutdown(registry.Context())

	engine, err := infra.NewSystemEngine(entities.MigrateMode, registry.PodmanConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(define.ExecErrorCodeGeneric)
	}
	defer engine.Shutdown(registry.Context())

	err = engine.Migrate(registry.Context(), cmd.Flags(), registry.PodmanConfig(), migrateOptions)
	if err != nil {
		fmt.Println(err)
		os.Exit(define.ExecErrorCodeGeneric)
	}
	os.Exit(0)
}
