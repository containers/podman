package main

import (
	"github.com/urfave/cli"
)

var (
	playSubCommands = []cli.Command{
		playKubeCommand,
	}

	playDescription = "Play a pod and its containers from a structured file."
	playCommand     = cli.Command{
		Name:                   "play",
		Usage:                  "play a container or pod",
		Description:            playDescription,
		ArgsUsage:              "",
		Subcommands:            playSubCommands,
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
		Hidden:                 true,
	}
)
