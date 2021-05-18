package manifest

import (
	"context"
	"fmt"

	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	manifestCreateOpts = entities.ManifestCreateOptions{}
	createCmd          = &cobra.Command{
		Use:               "create [options] LIST [IMAGE]",
		Short:             "Create manifest list or image index",
		Long:              "Creates manifest lists or image indexes.",
		RunE:              create,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman manifest create mylist:v1.11
  podman manifest create mylist:v1.11 arch-specific-image-to-add
  podman manifest create --all mylist:v1.11 transport:tagged-image-to-add`,
		Args: cobra.RangeArgs(1, 2),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCmd,
		Parent:  manifestCmd,
	})
	flags := createCmd.Flags()
	flags.BoolVar(&manifestCreateOpts.All, "all", false, "add all of the lists' images if the images to add are lists")
}

func create(cmd *cobra.Command, args []string) error {
	imageID, err := registry.ImageEngine().ManifestCreate(context.Background(), args[:1], args[1:], manifestCreateOpts)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", imageID)
	return nil
}
