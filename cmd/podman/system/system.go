package system

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

// Command: podman _system_
var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Manage podman",
	Long:  "Manage podman",
	RunE:  validate.SubCommandExists,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemCmd,
	})
}
