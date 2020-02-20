package main

import (
	"bytes"
	encjson "encoding/json"
	"fmt"

	"github.com/containers/buildah/util"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	manifestInspectCommand     cliconfig.ManifestInspectValues
	manifestInspectDescription = `display the contents of a manifest list or image index`
	_manifestInspectCommand    = &cobra.Command{
		Use:   "inspect [image]",
		Short: "manifest inspect",
		Long:  manifestInspectDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestInspectCommand.InputArgs = args
			manifestInspectCommand.GlobalFlags = MainGlobalOpts
			manifestInspectCommand.Remote = remoteclient
			return manifestInspectCmd(&manifestInspectCommand)
		},
		Example: `podman manifest inspect mylist:v1.11`,
		Args:    cobra.MinimumNArgs(1),
	}
)

func init() {
	manifestInspectCommand.Command = _manifestInspectCommand
	manifestInspectCommand.SetHelpTemplate(HelpTemplate())
	manifestInspectCommand.SetUsageTemplate(UsageTemplate())
}

func manifestInspectCmd(c *cliconfig.ManifestInspectValues) error {
	var imageSpec string
	args := c.InputArgs
	switch len(args) {
	case 0:
		return errors.New("At least a source list ID must be specified")
	case 1:
		imageSpec = args[0]
		if imageSpec == "" {
			return errors.Errorf(`Invalid image name "%s"`, imageSpec)
		}
	default:
		return errors.New("Only one argument is necessary for inspect: an image name")
	}

	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.DeferredShutdown(false)

	store := runtime.GetStore()
	systemContext := runtime.SystemContext()
	ref, _, err := util.FindImage(store, "", systemContext, imageSpec)
	if err != nil {
		if ref, err = alltransports.ParseImageName(imageSpec); err != nil {
			return err
		}
	}

	ctx := getContext()

	src, err := ref.NewImageSource(ctx, systemContext)
	if err != nil {
		return errors.Wrapf(err, "error reading image %q", transports.ImageName(ref))
	}
	defer src.Close()

	manifestBytes, manifestType, err := src.GetManifest(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "error loading manifest from image %q", transports.ImageName(ref))
	}
	if !manifest.MIMETypeIsMultiImage(manifestType) {
		return errors.Errorf("manifest from image %q is of type %q, which is not a list type", transports.ImageName(ref), manifestType)
	}

	var b bytes.Buffer
	err = encjson.Indent(&b, manifestBytes, "", "    ")
	if err != nil {
		return errors.Wrapf(err, "error rendering manifest for display")
	}

	fmt.Printf("%s\n", b.String())

	return nil
}
