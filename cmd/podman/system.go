package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var (
	systemDescription = "Manage podman"

	systemCommand = cliconfig.PodmanCommand{
		Command: &cobra.Command{
			Use:   "system",
			Short: "Manage podman",
			Long:  systemDescription,
			RunE:  commandRunE(),
		},
	}
)

var systemCommands = []*cobra.Command{
	_infoCommand,
}

func init() {
	systemCommand.AddCommand(systemCommands...)
	systemCommand.AddCommand(getSystemSubCommands()...)
	systemCommand.SetUsageTemplate(UsageTemplate())
}
