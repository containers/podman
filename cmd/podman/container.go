package main

import (
	"github.com/urfave/cli"
)

var (
	subCommands = []cli.Command{
		attachCommand,
		checkpointCommand,
		cleanupCommand,
		commitCommand,
		createCommand,
		diffCommand,
		execCommand,
		exportCommand,
		inspectCommand,
		killCommand,
		logsCommand,
		lsCommand,
		mountCommand,
		pauseCommand,
		portCommand,
		//		pruneCommand,
		refreshCommand,
		restartCommand,
		restoreCommand,
		rmCommand,
		runCommand,
		runlabelCommand,
		startCommand,
		statsCommand,
		stopCommand,
		topCommand,
		umountCommand,
		unpauseCommand,
		//		updateCommand,
		waitCommand,
	}

	containerDescription = "Manage containers"
	containerCommand     = cli.Command{
		Name:                   "container",
		Usage:                  "Manage Containers",
		Description:            containerDescription,
		ArgsUsage:              "",
		Subcommands:            subCommands,
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)
