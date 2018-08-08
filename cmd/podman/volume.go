package main

import (
	"github.com/urfave/cli"
)

var (
	volumeDescription = `Manage volumes.

Volumes are created in and can be shared between containers.`

	volumeSubCommands = []cli.Command{
		volumeCreateCommand,
		volumeLsCommand,
		volumeRmCommand,
		volumeInspectCommand,
		volumePruneCommand,
	}
	volumeCommand = cli.Command{
		Name:                   "volume",
		Usage:                  "Manage volumes",
		Description:            volumeDescription,
		UseShortOptionHandling: true,
		Subcommands:            volumeSubCommands,
	}
)
