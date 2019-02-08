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

func init() {
	containerCommand.AddCommand(getContainerSubCommands()...)
	rootCmd.AddCommand(containerCommand.Command)
}
