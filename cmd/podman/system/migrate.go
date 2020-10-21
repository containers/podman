// +build !remote

package system

import (
	"fmt"
	"os"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra"
	"github.com/spf13/cobra"
)

var (
	migrateDescription = `
        podman system migrate

        Migrate existing containers to a new version of Podman.
`

	migrateCommand = &cobra.Command{
		Use:   "migrate [options]",
		Args:  validate.NoArgs,
		Short: "Migrate containers",
		Long:  migrateDescription,
		Run:   migrate,
	}
)

var (
	migrateOptions entities.SystemMigrateOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: migrateCommand,
		Parent:  systemCmd,
	})

	flags := migrateCommand.Flags()
	flags.StringVar(&migrateOptions.NewRuntime, "new-runtime", "", "Specify a new runtime for all containers")
}

func migrate(cmd *cobra.Command, args []string) {
	// Shutdown all running engines, `renumber` will hijack repository
	registry.ContainerEngine().Shutdown(registry.Context())
	registry.ImageEngine().Shutdown(registry.Context())

	engine, err := infra.NewSystemEngine(entities.MigrateMode, registry.PodmanConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(125)
	}
	defer engine.Shutdown(registry.Context())

	err = engine.Migrate(registry.Context(), cmd.Flags(), registry.PodmanConfig(), migrateOptions)
	if err != nil {
		fmt.Println(err)
		os.Exit(125)
	}
	os.Exit(0)
}
