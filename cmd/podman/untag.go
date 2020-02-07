package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	untagCommand cliconfig.UntagValues

	_untagCommand = &cobra.Command{
		Use:   "untag [flags] IMAGE [NAME...]",
		Short: "Remove a name from a local image",
		Long:  "Removes one or more names from a locally-stored image.",
		RunE: func(cmd *cobra.Command, args []string) error {
			untagCommand.InputArgs = args
			untagCommand.GlobalFlags = MainGlobalOpts
			untagCommand.Remote = remoteclient
			return untag(&untagCommand)
		},
		Example: `podman untag 0e3bbc2
  podman untag imageID:latest otherImageName:latest
  podman untag httpd myregistryhost:5000/fedora/httpd:v2`,
	}
)

func init() {
	untagCommand.Command = _untagCommand
	untagCommand.SetHelpTemplate(HelpTemplate())
	untagCommand.SetUsageTemplate(UsageTemplate())
}

func untag(c *cliconfig.UntagValues) error {
	args := c.InputArgs

	if len(args) == 0 {
		return errors.Errorf("at least one image name needs to be specified")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.DeferredShutdown(false)

	imageName := args[0]
	newImage, err := runtime.NewImageFromLocal(imageName)
	if err != nil {
		return err
	}

	tags := args[1:]
	if len(args) == 1 {
		// Remove all tags if not explicitly specified
		tags = newImage.Names()
	}
	logrus.Debugf("Tags to be removed: %v", tags)

	for _, tag := range tags {
		if err := newImage.UntagImage(tag); err != nil {
			return errors.Wrapf(err, "removing %q from %q", tag, imageName)
		}
	}
	return nil
}
