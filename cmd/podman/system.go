package main

import (
	"sort"

	"github.com/urfave/cli"
)

var (
	systemSubCommands = []cli.Command{
		pruneSystemCommand,
	}
	systemDescription = "Manage podman"
	systemCommand     = cli.Command{
		Name:                   "system",
		Usage:                  "Manage podman",
		Description:            systemDescription,
		ArgsUsage:              "",
		Subcommands:            getSystemSubCommandsSorted(),
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

func getSystemSubCommandsSorted() []cli.Command {
	systemSubCommands = append(systemSubCommands, getSystemSubCommands()...)
	sort.Sort(commandSortedAlpha{systemSubCommands})
	return systemSubCommands
}
