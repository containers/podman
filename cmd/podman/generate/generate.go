package generate

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _generate_
	GenerateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate structured data based on containers, pods or volumes",
		Long:  "Generate structured data (e.g., Kubernetes YAML or systemd units) based on containers, pods or volumes.",
		RunE:  validate.SubCommandExists,
	}
	containerConfig = registry.PodmanConfig().ContainersConfDefaultsRO
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: GenerateCmd,
	})
}
