package main

import (
	"sort"

	"github.com/urfave/cli"
)

var (
	containerSubCommands = []cli.Command{
		inspectCommand,
	}
	containerDescription = "Manage containers"
	containerCommand     = cli.Command{
		Name:                   "container",
		Usage:                  "Manage Containers",
		Description:            containerDescription,
		ArgsUsage:              "",
		Subcommands:            getContainerSubCommandsSorted(),
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

func getContainerSubCommandsSorted() []cli.Command {
	containerSubCommands = append(containerSubCommands, getContainerSubCommands()...)
	sort.Sort(commandSortedAlpha{containerSubCommands})
	return containerSubCommands
}
