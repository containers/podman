package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var (
	generateCommand     cliconfig.PodmanCommand
	generateDescription = "Generate structured data based for a containers and pods"
	_generateCommand    = &cobra.Command{
		Use:   "generate",
		Short: "Generated structured data",
		Long:  generateDescription,
	}
)

func init() {
	generateCommand.Command = _generateCommand
	generateCommand.AddCommand(getGenerateSubCommands()...)
	generateCommand.SetUsageTemplate(UsageTemplate())
}
