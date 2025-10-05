//go:build !remote

package system

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
)

var (
	renumberDescription = `
        podman system renumber

        Migrate lock numbers to handle a change in maximum number of locks.
        Mandatory after the number of locks in containers.conf is changed.
`

	renumberCommand = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "renumber",
		Args:              validate.NoArgs,
		Short:             "Migrate lock numbers",
		Long:              renumberDescription,
		RunE:              renumber,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: renumberCommand,
		Parent:  systemCmd,
	})
}

func renumber(_ *cobra.Command, _ []string) error {
	return registry.ContainerEngine().Renumber(registry.Context())
}
