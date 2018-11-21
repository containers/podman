package main

import (
	"github.com/urfave/cli"
)

var (
	generateSubCommands = []cli.Command{
		containerKubeCommand,
	}

	generateDescription = "generate structured data based for a containers and pods"
	kubeCommand         = cli.Command{
		Name:                   "generate",
		Usage:                  "generated structured data",
		Description:            generateDescription,
		ArgsUsage:              "",
		Subcommands:            generateSubCommands,
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
		Hidden:                 true,
	}
)
