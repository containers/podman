package main

import (
	"sort"

	"github.com/urfave/cli"
)

var (
	imageSubCommands = []cli.Command{
		historyCommand,
		imageExistsCommand,
		inspectCommand,
		lsImagesCommand,
		pruneImagesCommand,
		pullCommand,
		rmImageCommand,
		tagCommand,
	}
	imageDescription = "Manage images"
	imageCommand     = cli.Command{
		Name:                   "image",
		Usage:                  "Manage images",
		Description:            imageDescription,
		ArgsUsage:              "",
		Subcommands:            getImageSubCommandsSorted(),
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

func getImageSubCommandsSorted() []cli.Command {
	imageSubCommands = append(imageSubCommands, getImageSubCommands()...)
	sort.Sort(commandSortedAlpha{imageSubCommands})
	return imageSubCommands
}
