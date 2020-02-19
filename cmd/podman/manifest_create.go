package main

import (
	"fmt"

	"github.com/containers/buildah/manifests"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/util"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	manifestCreateCommand     cliconfig.ManifestCreateValues
	manifestCreateDescription = `creates manifest lists and image indexes`
	_manifestCreateCommand    = &cobra.Command{
		Use:   "create [flags] [manifest] [tags]",
		Short: "manifest create",
		Long:  manifestCreateDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestCreateCommand.InputArgs = args
			manifestCreateCommand.GlobalFlags = MainGlobalOpts
			manifestCreateCommand.Remote = remoteclient
			return manifestCreateCmd(&manifestCreateCommand)
		},
		Example: `podman manifest create mylist:v1.11
  podman manifest create mylist:v1.11 arch-specific-image-to-add
  podman manifest create --all mylist:v1.11 transport:tagged-image-to-add`,
		Args: cobra.MinimumNArgs(1),
	}
)

func init() {
	manifestCreateCommand.Command = _manifestCreateCommand
	manifestCreateCommand.SetHelpTemplate(HelpTemplate())
	manifestCreateCommand.SetUsageTemplate(UsageTemplate())
	flags := manifestCreateCommand.Flags()
	flags.BoolVar(&manifestCreateCommand.All, "all", false, "add all of the lists' images if the images to add are lists")
	flags.StringVar(&manifestCreateCommand.OsOverride, "override-os", "", "if any of the specified images is a list, choose the one for `os`")
	if err := flags.MarkHidden("override-os"); err != nil {
		panic(fmt.Sprintf("error marking override-os as hidden: %v", err))
	}
	flags.StringVar(&manifestCreateCommand.ArchOverride, "override-arch", "", "if any of the specified images is a list, choose the one for `arch`")
	if err := flags.MarkHidden("override-arch"); err != nil {
		panic(fmt.Sprintf("error marking override-arch as hidden: %v", err))
	}
}

func manifestCreateCmd(c *cliconfig.ManifestCreateValues) error {
	args := c.InputArgs
	if len(args) == 0 {
		return errors.Errorf("At least a name must be specified for the list")
	}

	listImageSpec := args[0]
	imageSpecs := args[1:]

	store, err := getStore(c)
	if err != nil {
		return err
	}

	systemContext, err := parse.SystemContextFromOptions(c.Command)
	if err != nil {
		return errors.Wrapf(err, "error building system context")
	}

	list := manifests.Create()

	names, err := util.ExpandNames([]string{listImageSpec}, "", systemContext, store)
	if err != nil {
		return errors.Wrapf(err, "error encountered while expanding image name %q", listImageSpec)
	}

	for _, imageSpec := range imageSpecs {
		ref, _, err := util.FindImage(store, "", systemContext, imageSpec)
		if err != nil {
			if ref, err = alltransports.ParseImageName(imageSpec); err != nil {
				return err
			}
		}
		if _, err = list.Add(getContext(), systemContext, ref, c.All); err != nil {
			return err
		}
	}

	imageID, err := list.SaveToImage(store, "", names, manifest.DockerV2ListMediaType)
	if err == nil {
		fmt.Printf("%s\n", imageID)
	}
	return err
}
