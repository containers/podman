package images

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman image _list_
	listCmd = &cobra.Command{
		Use:     "list [flag] [IMAGE]",
		Aliases: []string{"ls"},
		Short:   "List images in local storage",
		Long:    "Lists images previously pulled to the system or created on the system.",
		RunE:    images,
		Example: `podman image list --format json
  podman image list --sort repository --format "table {{.ID}} {{.Repository}} {{.Tag}}"
  podman image list --filter dangling=true`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: listCmd,
		Parent:  imageCmd,
	})
}

func images(cmd *cobra.Command, args []string) error {
	_, _ = registry.Options(cmd)
	return nil
}
