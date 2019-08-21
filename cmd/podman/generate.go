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
	}
)

func init() {
	// Systemd-service generation is not supported for remote-clients.
	if !remoteclient {
		generateCommands = append(generateCommands, _containerSystemdCommand)
	}
	generateCommand.Command = _generateCommand
	generateCommand.AddCommand(generateCommands...)
	generateCommand.SetUsageTemplate(UsageTemplate())
}
