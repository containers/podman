package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var (
	imageDescription = "Manage images"
	imageCommand     = cliconfig.PodmanCommand{
		Command: &cobra.Command{
			Use:   "image",
			Short: "Manage images",
			Long:  imageDescription,
		},
	}
)

//imageSubCommands are implemented both in local and remote clients
var imageSubCommands = []*cobra.Command{
	_buildCommand,
	_historyCommand,
	_imageExistsCommand,
	_imagesCommand,
	_importCommand,
	_inspectCommand,
	_pruneImagesCommand,
	_pullCommand,
	_pushCommand,
	_rmiCommand,
	_saveCommand,
	_tagCommand,
}

func init() {
	imageCommand.SetUsageTemplate(UsageTemplate())
	imageCommand.AddCommand(imageSubCommands...)
	imageCommand.AddCommand(getImageSubCommands()...)
}
