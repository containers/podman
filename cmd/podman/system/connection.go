package system

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// ConnectionCmd skips creating engines (PersistentPreRunE/PersistentPostRunE are No-Op's) since
	// sub-commands will obtain connection information to said engines
	ConnectionCmd = &cobra.Command{
		Use:                "connection",
		Short:              "Manage remote API service destinations",
		Long:               `Manage remote API service destination information in podman configuration`,
		PersistentPreRunE:  validate.NoOp,
		RunE:               validate.SubCommandExists,
		PersistentPostRunE: validate.NoOp,
		TraverseChildren:   false,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: ConnectionCmd,
		Parent:  systemCmd,
	})
}
