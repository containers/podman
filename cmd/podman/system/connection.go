package system

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Skip creating engines since this command will obtain connection information to said engines
	noOp = func(cmd *cobra.Command, args []string) error {
		return nil
	}

	ConnectionCmd = &cobra.Command{
		Use:                "connection",
		Short:              "Manage remote ssh destinations",
		Long:               `Manage ssh destination information in podman configuration`,
		PersistentPreRunE:  noOp,
		RunE:               validate.SubCommandExists,
		PersistentPostRunE: noOp,
		TraverseChildren:   false,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: ConnectionCmd,
		Parent:  systemCmd,
	})
}
