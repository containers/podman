package pods

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	psDescription = "List all pods on system including their names, ids and current state."

	// Command: podman pod _ps_
	psCmd = &cobra.Command{
		Use:     "ps",
		Aliases: []string{"ls", "list"},
		Short:   "list pods",
		Long:    psDescription,
		RunE:    pods,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: psCmd,
		Parent:  podCmd,
	})
}

func pods(cmd *cobra.Command, args []string) error {
	return nil
}
