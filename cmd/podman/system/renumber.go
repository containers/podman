// +build !remote

package system

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra"
	"github.com/spf13/cobra"
)

var (
	renumberDescription = `
        podman system renumber

        Migrate lock numbers to handle a change in maximum number of locks.
        Mandatory after the number of locks in containers.conf is changed.
`

	renumberCommand = &cobra.Command{
		Use:                   "renumber",
		Args:                  validate.NoArgs,
		DisableFlagsInUseLine: true,
		Short:                 "Migrate lock numbers",
		Long:                  renumberDescription,
		Run:                   renumber,
		ValidArgsFunction:     completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: renumberCommand,
		Parent:  systemCmd,
	})
}
func renumber(cmd *cobra.Command, args []string) {
	// Shutdown all running engines, `renumber` will hijack all methods
	registry.ContainerEngine().Shutdown(registry.Context())
	registry.ImageEngine().Shutdown(registry.Context())

	engine, err := infra.NewSystemEngine(entities.RenumberMode, registry.PodmanConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(125)
	}
	defer engine.Shutdown(registry.Context())

	err = engine.Renumber(registry.Context(), cmd.Flags(), registry.PodmanConfig())
	if err != nil {
		fmt.Println(err)
		os.Exit(125)
	}
	os.Exit(0)
}
