package generate

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/util"
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
	containerConfig = util.DefaultContainerConfig()
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: GenerateCmd,
	})
}
