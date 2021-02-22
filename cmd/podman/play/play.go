package pods

import (
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _play_
	playCmd = &cobra.Command{
		Use:   "play",
		Short: "Play a pod and its containers from a structured file.",
		Long:  "Play structured data (e.g., Kubernetes pod or service yaml) based on containers and pods.",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: playCmd,
	})
}
