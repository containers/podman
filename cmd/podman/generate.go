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
		RunE:  commandRunE(),
	}

	//	Commands that are universally implemented
	generateCommands = []*cobra.Command{
		_containerKubeCommand,
		_containerSystemdCommand,
	}
)

func init() {
	generateCommand.Command = _generateCommand
	generateCommand.AddCommand(generateCommands...)
	generateCommand.SetUsageTemplate(UsageTemplate())
}
