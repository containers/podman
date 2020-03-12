package containers

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// podman container _list_
	listCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List containers",
		Long:    "Prints out information about the containers",
		RunE:    containers,
		Example: `podman container list -a
  podman container list -a --format "{{.ID}}  {{.Image}}  {{.Labels}}  {{.Mounts}}"
  podman container list --size --sort names`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: listCmd,
		Parent:  containerCmd,
	})
}

func containers(cmd *cobra.Command, args []string) error {
	return nil
}
