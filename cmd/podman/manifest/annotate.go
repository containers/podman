package manifest

import (
	"context"
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	manifestAnnotateOpts = entities.ManifestAnnotateOptions{}
	annotateCmd          = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "annotate [options] LIST IMAGE",
		Short:             "Add or update information about an entry in a manifest list or image index",
		Long:              "Adds or updates information about an entry in a manifest list or image index.",
		RunE:              annotate,
		Example:           `podman manifest annotate --annotation left=right mylist:v1.11 image:v1.11-amd64`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteImages,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: annotateCmd,
		Parent:  manifestCmd,
	})
	flags := annotateCmd.Flags()

	annotationFlagName := "annotation"
	flags.StringSliceVar(&manifestAnnotateOpts.Annotation, annotationFlagName, nil, "set an `annotation` for the specified image")
	_ = annotateCmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)

	archFlagName := "arch"
	flags.StringVar(&manifestAnnotateOpts.Arch, archFlagName, "", "override the `architecture` of the specified image")
	_ = annotateCmd.RegisterFlagCompletionFunc(archFlagName, completion.AutocompleteArch)

	featuresFlagName := "features"
	flags.StringSliceVar(&manifestAnnotateOpts.Features, featuresFlagName, nil, "override the `features` of the specified image")
	_ = annotateCmd.RegisterFlagCompletionFunc(featuresFlagName, completion.AutocompleteNone)

	osFlagName := "os"
	flags.StringVar(&manifestAnnotateOpts.OS, osFlagName, "", "override the `OS` of the specified image")
	_ = annotateCmd.RegisterFlagCompletionFunc(osFlagName, completion.AutocompleteOS)

	osFeaturesFlagName := "os-features"
	flags.StringSliceVar(&manifestAnnotateOpts.OSFeatures, osFeaturesFlagName, nil, "override the OS `features` of the specified image")
	_ = annotateCmd.RegisterFlagCompletionFunc(osFeaturesFlagName, completion.AutocompleteNone)

	osVersionFlagName := "os-version"
	flags.StringVar(&manifestAnnotateOpts.OSVersion, osVersionFlagName, "", "override the OS `version` of the specified image")
	_ = annotateCmd.RegisterFlagCompletionFunc(osVersionFlagName, completion.AutocompleteNone)

	variantFlagName := "variant"
	flags.StringVar(&manifestAnnotateOpts.Variant, variantFlagName, "", "override the `Variant` of the specified image")
	_ = annotateCmd.RegisterFlagCompletionFunc(variantFlagName, completion.AutocompleteNone)
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
		return err
	}
	fmt.Printf("%s\n", updatedListID)
	return nil
}
