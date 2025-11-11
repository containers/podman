package secrets

import (
	"github.com/containers/podman/v6/cmd/podman/registry"
	"github.com/containers/podman/v6/cmd/podman/validate"
	"github.com/spf13/cobra"
)

// Command: podman _secret_
var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets",
	Long:  "Manage secrets",
	RunE:  validate.SubCommandExists,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: secretCmd,
	})
}
