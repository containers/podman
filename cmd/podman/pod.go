package main

import (
	"github.com/urfave/cli"
)

var (
	podDescription = `
   podman pod

   manage pods
`
	podCommand = cli.Command{
		Name:                   "pod",
		Usage:                  "Manage pods",
		Description:            podDescription,
		UseShortOptionHandling: true,
		Subcommands: []cli.Command{
			podStartCommand,
			podCreateCommand,
			podPsCommand,
			podRmCommand,
		},
	}
)
