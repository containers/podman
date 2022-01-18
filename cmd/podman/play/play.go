package pods

import (
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _play_
	playCmd = &cobra.Command{
		Use:   "play",
		Short: "Play containers, pods or volumes from a structured file",
		Long:  "Play structured data (e.g., Kubernetes YAML) based on containers, pods or volumes.",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: playCmd,
	})
}
