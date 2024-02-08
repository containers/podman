package images

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	untagCmd = &cobra.Command{
		Use:               "untag IMAGE [IMAGE...]",
		Short:             "Remove a name from a local image",
		Long:              "Removes one or more names from a locally-stored image.",
		RunE:              untag,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman untag 0e3bbc2
  podman untag imageID:latest otherImageName:latest
  podman untag httpd myregistryhost:5000/fedora/httpd:v2`,
	}

	imageUntagCmd = &cobra.Command{
		Args:              untagCmd.Args,
		Use:               untagCmd.Use,
		Short:             untagCmd.Short,
		Long:              untagCmd.Long,
		RunE:              untagCmd.RunE,
		ValidArgsFunction: untagCmd.ValidArgsFunction,
		Example: `podman image untag 0e3bbc2
  podman image untag imageID:latest otherImageName:latest
  podman image untag httpd myregistryhost:5000/fedora/httpd:v2`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: untagCmd,
	})
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageUntagCmd,
		Parent:  imageCmd,
	})
}

func untag(cmd *cobra.Command, args []string) error {
	return registry.ImageEngine().Untag(registry.GetContext(), args[0], args[1:], entities.ImageUntagOptions{})
}
