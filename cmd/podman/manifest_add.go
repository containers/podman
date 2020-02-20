package main

import (
	"fmt"
	"strings"

	"github.com/containers/buildah/manifests"
	"github.com/containers/buildah/util"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	manifestAddCommand     cliconfig.ManifestAddValues
	manifestAddDescription = `adds an image to a manifest list or image index`
	_manifestAddCommand    = &cobra.Command{
		Use:   "add [manifest] [tags]",
		Short: "manifest add",
		Long:  manifestAddDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestAddCommand.InputArgs = args
			manifestAddCommand.GlobalFlags = MainGlobalOpts
			manifestAddCommand.Remote = remoteclient
			return manifestAddCmd(&manifestAddCommand)
		},
		Example: `podman manifest add mylist:v1.11 image:v1.11-amd64
  podman manifest add mylist:v1.11 transport:imageName`,
		Args: cobra.MinimumNArgs(2),
	}
)

func init() {
	manifestAddCommand.Command = _manifestAddCommand
	manifestAddCommand.SetHelpTemplate(HelpTemplate())
	manifestAddCommand.SetUsageTemplate(UsageTemplate())
	flags := manifestAddCommand.Flags()
	flags.StringVar(&manifestAddCommand.OsOverride, "override-os", "", "if any of the specified images is a list, choose the one for `os`")
	if err := flags.MarkHidden("override-os"); err != nil {
		panic(fmt.Sprintf("error marking override-os as hidden: %v", err))
	}
	flags.StringVar(&manifestAddCommand.ArchOverride, "override-arch", "", "if any of the specified images is a list, choose the one for `arch`")
	if err := flags.MarkHidden("override-arch"); err != nil {
		panic(fmt.Sprintf("error marking override-arch as hidden: %v", err))
	}
	flags.StringVar(&manifestAddCommand.Os, "os", "", "override the `OS` of the specified image")
	flags.StringVar(&manifestAddCommand.Arch, "arch", "", "override the `architecture` of the specified image")
	flags.StringVar(&manifestAddCommand.Variant, "variant", "", "override the `Variant` of the specified image")
	flags.StringVar(&manifestAddCommand.OsVersion, "os-version", "", "override the OS `version` of the specified image")
	flags.StringSliceVar(&manifestAddCommand.Features, "features", nil, "override the `features` of the specified image")
	flags.StringSliceVar(&manifestAddCommand.OsFeatures, "os-features", nil, "override the OS `features` of the specified image")
	flags.StringSliceVar(&manifestAddCommand.Annotations, "annotation", nil, "set an `annotation` for the specified image")
	flags.BoolVar(&manifestAddCommand.All, "all", false, "add all of the list's images if the image is a list")
}

func manifestAddCmd(c *cliconfig.ManifestAddValues) error {
	listImageSpec := ""
	imageSpec := ""
	args := c.InputArgs
	switch len(args) {
	case 0, 1:
		return errors.New("At least a list image and an image to add must be specified")
	case 2:
		listImageSpec = args[0]
		if listImageSpec == "" {
			return errors.Errorf(`Invalid image name "%s"`, args[0])
		}
		imageSpec = args[1]
		if imageSpec == "" {
			return errors.Errorf(`Invalid image name "%s"`, args[1])
		}
	default:
		return errors.New("At least two arguments are necessary: list and image to add to list")
	}

	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.DeferredShutdown(false)

	store := runtime.GetStore()
	systemContext := runtime.SystemContext()

	_, listImage, err := util.FindImage(store, "", systemContext, listImageSpec)
	if err != nil {
		return err
	}

	ref, _, err := util.FindImage(store, "", systemContext, imageSpec)
	if err != nil {
		if ref, err = alltransports.ParseImageName(imageSpec); err != nil {
			return err
		}
	}

	_, list, err := manifests.LoadFromImage(store, listImage.ID)
	if err != nil {
		return err
	}

	ctx := getContext()

	digest, err := list.Add(ctx, systemContext, ref, c.All)
	if err != nil {
		return err
	}

	if c.IsSet("os") {
		if err := list.SetOS(digest, c.Os); err != nil {
			return err
		}
	}
	if c.IsSet("os-version") {
		if err := list.SetOSVersion(digest, c.OsVersion); err != nil {
			return err
		}
	}
	if c.IsSet("os-features") {
		if err := list.SetOSFeatures(digest, c.OsFeatures); err != nil {
			return err
		}
	}
	if c.IsSet("arch") {
		if err := list.SetArchitecture(digest, c.Arch); err != nil {
			return err
		}
	}
	if c.IsSet("variant") {
		if err := list.SetVariant(digest, c.Variant); err != nil {
			return err
		}
	}
	if len(c.Features) != 0 {
		if err := list.SetFeatures(digest, c.Features); err != nil {
			return err
		}
	}
	if len(c.Annotations) != 0 {
		annotations := make(map[string]string)
		for _, annotationSpec := range c.Annotations {
			spec := strings.SplitN(annotationSpec, "=", 2)
			if len(spec) != 2 {
				return errors.Errorf("no value given for annotation %q", spec[0])
			}
			annotations[spec[0]] = spec[1]
		}
		if err := list.SetAnnotations(&digest, annotations); err != nil {
			return err
		}
	}

	updatedListID, err := list.SaveToImage(store, listImage.ID, nil, "")
	if err == nil {
		fmt.Printf("%s: %s\n", updatedListID, digest.String())
	}

	return err
}
