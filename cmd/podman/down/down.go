package down

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _down_
	downCmd = &cobra.Command{
		Use:   "down",
		Short: "Stop containers, pods or volumes from a structured file",
		Long:  "Stop containers, pods or volumes created from a structured file (e.g., Kubernetes YAML).",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: downCmd,
	})
}
