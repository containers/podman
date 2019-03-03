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
	imagesSubCommand  cliconfig.ImagesValues
	_imagesSubCommand = &cobra.Command{
		Use:     strings.Replace(_imagesCommand.Use, "images", "list", 1),
		Short:   _imagesCommand.Short,
		Long:    _imagesCommand.Long,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			imagesSubCommand.InputArgs = args
			imagesSubCommand.GlobalFlags = MainGlobalOpts
			return imagesCmd(&imagesSubCommand)
		},
		Example: strings.Replace(_imagesCommand.Example, "podman images", "podman image list", -1),
	}

	rmSubCommand  cliconfig.RmiValues
	_rmSubCommand = &cobra.Command{
		Use:   strings.Replace(_rmiCommand.Use, "rmi", "rm", 1),
		Short: _rmiCommand.Short,
		Long:  _rmiCommand.Long,
		RunE: func(cmd *cobra.Command, args []string) error {
			rmSubCommand.InputArgs = args
			rmSubCommand.GlobalFlags = MainGlobalOpts
			return rmiCmd(&rmSubCommand)
		},
		Example: strings.Replace(_rmiCommand.Example, "podman rmi", "podman image rm", -1),
	}
)

//imageSubCommands are implemented both in local and remote clients
var imageSubCommands = []*cobra.Command{
	_buildCommand,
	_historyCommand,
	_imagesSubCommand,
	_imageExistsCommand,
	_importCommand,
	_inspectCommand,
	_loadCommand,
	_pruneImagesCommand,
	_pullCommand,
	_pushCommand,
	_rmSubCommand,
	_saveCommand,
	_tagCommand,
}

func init() {
	rmSubCommand.Command = _rmSubCommand
	rmiInit(&rmSubCommand)

	imagesSubCommand.Command = _imagesSubCommand
	imagesInit(&imagesSubCommand)

	imageCommand.SetUsageTemplate(UsageTemplate())
	imageCommand.AddCommand(imageSubCommands...)
	imageCommand.AddCommand(getImageSubCommands()...)

}
