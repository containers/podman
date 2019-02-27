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
	_imagesSubCommand = _imagesCommand
	_rmSubCommand     = _rmiCommand
)

//imageSubCommands are implemented both in local and remote clients
var imageSubCommands = []*cobra.Command{
	_buildCommand,
	_historyCommand,
	_imageExistsCommand,
	_importCommand,
	_inspectCommand,
	_loadCommand,
	_pruneImagesCommand,
	_pullCommand,
	_pushCommand,
	_saveCommand,
	_tagCommand,
}

func init() {
	imageCommand.SetUsageTemplate(UsageTemplate())
	imageCommand.AddCommand(imageSubCommands...)
	imageCommand.AddCommand(getImageSubCommands()...)

	// Setup of "images" to appear as "list"
	_imagesSubCommand.Use = "list"
	_imagesSubCommand.Aliases = []string{"ls"}
	imageCommand.AddCommand(&_imagesSubCommand)

	// Setup of "rmi" to appears as "rm"
	_rmSubCommand.Use = "rm"
	imageCommand.AddCommand(&_rmSubCommand)
}
