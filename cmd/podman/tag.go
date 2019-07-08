package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	tagCommand cliconfig.TagValues

	tagDescription = "Adds one or more additional names to locally-stored image."
	_tagCommand    = &cobra.Command{
		Use:   "tag [flags] IMAGE TAG [TAG...]",
		Short: "Add an additional name to a local image",
		Long:  tagDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			tagCommand.InputArgs = args
			tagCommand.GlobalFlags = MainGlobalOpts
			tagCommand.Remote = remoteclient
			return tagCmd(&tagCommand)
		},
		Example: `podman tag 0e3bbc2 fedora:latest
  podman tag imageID:latest myNewImage:newTag
  podman tag httpd myregistryhost:5000/fedora/httpd:v2`,
	}
)

func init() {
	tagCommand.Command = _tagCommand
	tagCommand.SetHelpTemplate(HelpTemplate())
	tagCommand.SetUsageTemplate(UsageTemplate())
}

func tagCmd(c *cliconfig.TagValues) error {
	args := c.InputArgs
	if len(args) < 2 {
		return errors.Errorf("image name and at least one new name must be specified")
	}
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.DeferredShutdown(false)

	newImage, err := runtime.NewImageFromLocal(args[0])
	if err != nil {
		return err
	}

	for _, tagName := range args[1:] {
		if err := newImage.TagImage(tagName); err != nil {
			return errors.Wrapf(err, "error adding '%s' to image %q", tagName, newImage.InputName)
		}
	}
	return nil
}
