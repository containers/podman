package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var containerDescription = "Manage containers"
var containerCommand = cliconfig.PodmanCommand{
	Command: &cobra.Command{
		Use:              "container",
		Short:            "Manage Containers",
		Long:             containerDescription,
		TraverseChildren: true,
	},
}

// Commands that are universally implemented.
var containerCommands = []*cobra.Command{
	_containerExistsCommand,
}

func init() {
	containerCommand.AddCommand(containerCommands...)
	containerCommand.AddCommand(getContainerSubCommands()...)
	containerCommand.SetUsageTemplate(UsageTemplate())
	rootCmd.AddCommand(containerCommand.Command)
}
