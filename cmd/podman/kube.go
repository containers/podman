package main

import (
	"github.com/urfave/cli"
)

var (
	kubeSubCommands = []cli.Command{
		containerKubeCommand,
	}

	kubeDescription = "Work with Kubernetes objects"
	kubeCommand     = cli.Command{
		Name:                   "kube",
		Usage:                  "Import and export Kubernetes objections from and to Podman",
		Description:            containerDescription,
		ArgsUsage:              "",
		Subcommands:            kubeSubCommands,
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
		Hidden:                 true,
	}
)
