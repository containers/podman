package images

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	tagDescription = "Adds one or more additional names to locally-stored image."
	tagCommand     = &cobra.Command{
		Use:               "tag IMAGE TARGET_NAME [TARGET_NAME...]",
		Short:             "Add an additional name to a local image",
		Long:              tagDescription,
		RunE:              tag,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman tag 0e3bbc2 fedora:latest
  podman tag imageID:latest myNewImage:newTag
  podman tag httpd myregistryhost:5000/fedora/httpd:v2`,
	}

	imageTagCommand = &cobra.Command{
		Args:              tagCommand.Args,
		Use:               tagCommand.Use,
		Short:             tagCommand.Short,
		Long:              tagCommand.Long,
		RunE:              tagCommand.RunE,
		ValidArgsFunction: tagCommand.ValidArgsFunction,
		Example: `podman image tag 0e3bbc2 fedora:latest
  podman image tag imageID:latest myNewImage:newTag
  podman image tag httpd myregistryhost:5000/fedora/httpd:v2`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: tagCommand,
	})
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageTagCommand,
		Parent:  imageCmd,
	})
}

func tag(cmd *cobra.Command, args []string) error {
	return registry.ImageEngine().Tag(registry.GetContext(), args[0], args[1:], entities.ImageTagOptions{})
}
