package manifest

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	manifestAddOpts = entities.ManifestAddOptions{}
	addCmd          = &cobra.Command{
		Use:   "add",
		Short: "Add images to a manifest list or image index",
		Long:  "Adds an image to a manifest list or image index.",
		RunE:  add,
		Example: `podman manifest add mylist:v1.11 image:v1.11-amd64
		podman manifest add mylist:v1.11 transport:imageName`,
		Args: cobra.ExactArgs(2),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: addCmd,
		Parent:  manifestCmd,
	})
	flags := addCmd.Flags()
	flags.BoolVar(&manifestAddOpts.All, "all", false, "add all of the list's images if the image is a list")
	flags.StringSliceVar(&manifestAddOpts.Annotation, "annotation", nil, "set an `annotation` for the specified image")
	flags.StringVar(&manifestAddOpts.Arch, "arch", "", "override the `architecture` of the specified image")
	flags.StringSliceVar(&manifestAddOpts.Features, "features", nil, "override the `features` of the specified image")
	flags.StringVar(&manifestAddOpts.OSVersion, "os-version", "", "override the OS `version` of the specified image")
	flags.StringVar(&manifestAddOpts.Variant, "variant", "", "override the `Variant` of the specified image")
}

func add(cmd *cobra.Command, args []string) error {
	manifestAddOpts.Images = []string{args[1], args[0]}
	listID, err := registry.ImageEngine().ManifestAdd(context.Background(), manifestAddOpts)
	if err != nil {
		return errors.Wrapf(err, "error adding to manifest list %s", args[0])
	}
	fmt.Printf("%s\n", listID)
	return nil
}
