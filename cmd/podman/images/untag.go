package images

import (
	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	untagCommand = &cobra.Command{
		Use:                   "untag IMAGE [IMAGE...]",
		Short:                 "Remove a name from a local image",
		Long:                  "Removes one or more names from a locally-stored image.",
		RunE:                  untag,
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		ValidArgsFunction:     common.AutocompleteImages,
		Example: `podman untag 0e3bbc2
  podman untag imageID:latest otherImageName:latest
  podman untag httpd myregistryhost:5000/fedora/httpd:v2`,
	}

	imageUntagCommand = &cobra.Command{
		Args:                  untagCommand.Args,
		DisableFlagsInUseLine: true,
		Use:                   untagCommand.Use,
		Short:                 untagCommand.Short,
		Long:                  untagCommand.Long,
		RunE:                  untagCommand.RunE,
		ValidArgsFunction:     untagCommand.ValidArgsFunction,
		Example: `podman image untag 0e3bbc2
  podman image untag imageID:latest otherImageName:latest
  podman image untag httpd myregistryhost:5000/fedora/httpd:v2`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: untagCommand,
	})
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imageUntagCommand,
		Parent:  imageCmd,
	})
}

func untag(cmd *cobra.Command, args []string) error {
	return registry.ImageEngine().Untag(registry.GetContext(), args[0], args[1:], entities.ImageUntagOptions{})
}
