package main

import (
	"github.com/urfave/cli"
)

var (
	podDescription = `
   podman pod

   Manage container pods.
   Pods are a group of one or more containers sharing the same network, pid and ipc namespaces.
`
	podCommand = cli.Command{
		Name:                   "pod",
		Usage:                  "Manage pods",
		Description:            podDescription,
		UseShortOptionHandling: true,
		Subcommands: []cli.Command{
			podCreateCommand,
			podPsCommand,
			podRestartCommand,
			podRmCommand,
			podStartCommand,
			podStopCommand,
		},
	}
)
