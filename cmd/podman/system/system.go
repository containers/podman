package system

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _system_
	systemCmd = &cobra.Command{
		Use:   "system",
		Short: "Manage podman",
		Long:  "Manage podman",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemCmd,
	})
}
