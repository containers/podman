package main

import (
	"strings"

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
	_imagesSubCommand.Use = strings.Replace(_imagesSubCommand.Use, "images", "list", 1)
	_imagesSubCommand.Aliases = []string{"ls"}
	_imagesSubCommand.Example = strings.Replace(_imagesSubCommand.Example, "podman images", "podman image list", -1)
	imageCommand.AddCommand(&_imagesSubCommand)

	// It makes no sense to keep 'podman images rmi'; just use 'rm'
	_rmSubCommand.Use = strings.Replace(_rmSubCommand.Use, "rmi", "rm", 1)
	_rmSubCommand.Example = strings.Replace(_rmSubCommand.Example, "podman rmi", "podman image rm", -1)
	imageCommand.AddCommand(&_rmSubCommand)
}
