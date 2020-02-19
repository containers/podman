package main

import (
	"fmt"
	"strings"

	"github.com/containers/buildah/manifests"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/util"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	manifestAnnotateCommand     cliconfig.ManifestAnnotateValues
	manifestAnnotateDescription = `adds or updates information about an entry in a manifest list or image index`
	_manifestAnnotateCommand    = &cobra.Command{
		Use:   "annotate [flags] [manifest] [tags]",
		Short: "Add or update information about an entry in a manifest list or image index",
		Long:  manifestAnnotateDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestAnnotateCommand.InputArgs = args
			manifestAnnotateCommand.GlobalFlags = MainGlobalOpts
			manifestAnnotateCommand.Remote = remoteclient
			return manifestAnnotateCmd(&manifestAnnotateCommand)
		},
		Example: `podman manifest annotate --annotation left=right mylist:v1.11 image:v1.11-amd64`,
		Args:    cobra.MinimumNArgs(2),
	}
)

func init() {
	manifestAnnotateCommand.Command = _manifestAnnotateCommand
	manifestAnnotateCommand.SetHelpTemplate(HelpTemplate())
	manifestAnnotateCommand.SetUsageTemplate(UsageTemplate())
	flags := manifestAnnotateCommand.Flags()
	flags.StringVar(&manifestAnnotateCommand.Os, "os", "", "override the `OS` of the specified image")
	flags.StringVar(&manifestAnnotateCommand.Arch, "arch", "", "override the `Architecture` of the specified image")
	flags.StringVar(&manifestAnnotateCommand.Variant, "variant", "", "override the `Variant` of the specified image")
	flags.StringVar(&manifestAnnotateCommand.OsVersion, "os-version", "", "override the os `version` of the specified image")
	flags.StringSliceVar(&manifestAnnotateCommand.Features, "features", nil, "override the `features` of the specified image")
	flags.StringSliceVar(&manifestAnnotateCommand.OsFeatures, "os-features", nil, "override the os `features` of the specified image")
	flags.StringSliceVar(&manifestAnnotateCommand.Annotations, "annotation", nil, "set an `annotation` for the specified image")
}

func manifestAnnotateCmd(c *cliconfig.ManifestAnnotateValues) error {
	listImageSpec := ""
	imageSpec := ""
	args := c.InputArgs
	switch len(args) {
	case 0:
		return errors.New("At least a list image must be specified")
	case 1:
		listImageSpec = args[0]
		if listImageSpec == "" {
			return errors.Errorf(`Invalid image name "%s"`, args[0])
		}
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

	store, err := getStore(c)
	if err != nil {
		return err
	}

	systemContext, err := parse.SystemContextFromOptions(c.Command)
	if err != nil {
		return errors.Wrapf(err, "error building system context")
	}

	_, listImage, err := util.FindImage(store, "", systemContext, listImageSpec)
	if err != nil {
		return err
	}

	_, list, err := manifests.LoadFromImage(store, listImage.ID)
	if err != nil {
		return err
	}

	ctx := getContext()

	digest, err := digest.Parse(imageSpec)
	if err != nil {
		ref, _, err := util.FindImage(store, "", systemContext, imageSpec)
		if err != nil {
			return err
		}
		img, err := ref.NewImageSource(ctx, systemContext)
		if err != nil {
			return err
		}
		defer img.Close()
		manifestBytes, _, err := img.GetManifest(ctx, nil)
		if err != nil {
			return err
		}
		digest, err = manifest.Digest(manifestBytes)
		if err != nil {
			return err
		}
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

	return nil
}
