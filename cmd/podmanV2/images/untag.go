package images

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	untagCommand = &cobra.Command{
		Use:   "untag [flags] IMAGE [NAME...]",
		Short: "Remove a name from a local image",
		Long:  "Removes one or more names from a locally-stored image.",
		RunE:  untag,
		Args:  cobra.MinimumNArgs(1),
		Example: `podman untag 0e3bbc2
  podman untag imageID:latest otherImageName:latest
  podman untag httpd myregistryhost:5000/fedora/httpd:v2`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: untagCommand,
	})
	untagCommand.SetHelpTemplate(registry.HelpTemplate())
	untagCommand.SetUsageTemplate(registry.UsageTemplate())
}

func untag(cmd *cobra.Command, args []string) error {
	return registry.ImageEngine().Untag(registry.GetContext(), args[0], args[1:], entities.ImageUntagOptions{})
}
