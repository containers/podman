package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var generateDescription = "Generate structured data based for a containers and pods"
var generateCommand = cliconfig.PodmanCommand{

	Command: &cobra.Command{
		Use:   "generate",
		Short: "Generated structured data",
		Long:  generateDescription,
	},
}

func init() {
	generateCommand.AddCommand(getGenerateSubCommands()...)
}
