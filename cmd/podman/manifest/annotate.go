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
	manifestAnnotateOpts = entities.ManifestAnnotateOptions{}
	annotateCmd          = &cobra.Command{
		Use:     "annotate [flags] LIST IMAGE",
		Short:   "Add or update information about an entry in a manifest list or image index",
		Long:    "Adds or updates information about an entry in a manifest list or image index.",
		RunE:    annotate,
		Example: `podman manifest annotate --annotation left=right mylist:v1.11 image:v1.11-amd64`,
		Args:    cobra.ExactArgs(2),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: annotateCmd,
		Parent:  manifestCmd,
	})
	flags := annotateCmd.Flags()
	flags.StringSliceVar(&manifestAnnotateOpts.Annotation, "annotation", nil, "set an `annotation` for the specified image")
	flags.StringVar(&manifestAnnotateOpts.Arch, "arch", "", "override the `architecture` of the specified image")
	flags.StringSliceVar(&manifestAnnotateOpts.Features, "features", nil, "override the `features` of the specified image")
	flags.StringVar(&manifestAnnotateOpts.OS, "os", "", "override the `OS` of the specified image")
	flags.StringSliceVar(&manifestAnnotateOpts.OSFeatures, "os-features", nil, "override the OS `features` of the specified image")
	flags.StringVar(&manifestAnnotateOpts.OSVersion, "os-version", "", "override the OS `version` of the specified image")
	flags.StringVar(&manifestAnnotateOpts.Variant, "variant", "", "override the `variant` of the specified image")
}

func annotate(cmd *cobra.Command, args []string) error {
	listImageSpec := args[0]
	instanceSpec := args[1]
	if listImageSpec == "" {
		return errors.Errorf(`invalid image name "%s"`, listImageSpec)
	}
	if instanceSpec == "" {
		return errors.Errorf(`invalid image digest "%s"`, instanceSpec)
	}
	updatedListID, err := registry.ImageEngine().ManifestAnnotate(context.Background(), args, manifestAnnotateOpts)
	if err != nil {
		return errors.Wrapf(err, "error removing from manifest list %s", listImageSpec)
	}
	fmt.Printf("%s\n", updatedListID)
	return nil
}
