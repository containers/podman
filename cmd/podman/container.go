package main

import (
	"github.com/urfave/cli"
)

var (
	subCommands = []cli.Command{
		attachCommand,
		cleanupCommand,
		commitCommand,
		createCommand,
		diffCommand,
		execCommand,
		exportCommand,
		inspectCommand,
		installCommand,
		killCommand,
		logsCommand,
		lsCommand,
		mountCommand,
		pauseCommand,
		portCommand,
		//		pruneCommand,
		refreshCommand,
		restartCommand,
		rmCommand,
		runCommand,
		startCommand,
		statsCommand,
		stopCommand,
		topCommand,
		umountCommand,
		uninstallCommand,
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
