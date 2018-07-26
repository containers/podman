package main

import (
	"github.com/urfave/cli"
)

var (
	podDescription = `Manage container pods.

Pods are a group of one or more containers sharing the same network, pid and ipc namespaces.
`
	podSubCommands = []cli.Command{
		podCreateCommand,
		podKillCommand,
		podPsCommand,
		podRestartCommand,
		podRmCommand,
		podStartCommand,
		podStopCommand,
	}
	podCommand = cli.Command{
		Name:                   "pod",
		Usage:                  "Manage pods",
		Description:            podDescription,
		UseShortOptionHandling: true,
		Subcommands:            podSubCommands,
	}
)
