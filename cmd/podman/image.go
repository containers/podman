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
	_historyCommand,
	_imageExistsCommand,
	_inspectCommand,
	_imagesCommand,
	_pruneImagesCommand,
	_pushCommand,
	_rmiCommand,
	_tagCommand,
}

func init() {
	imageCommand.AddCommand(imageSubCommands...)
	imageCommand.AddCommand(getImageSubCommands()...)
	rootCmd.AddCommand(imageCommand.Command)
}
