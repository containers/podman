//+build !remoteclient

package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var networkcheckDescription = "Manage networks"
var networkcheckCommand = cliconfig.PodmanCommand{
	Command: &cobra.Command{
		Use:   "network",
		Short: "Manage Networks",
		Long:  networkcheckDescription,
		RunE:  commandRunE(),
	},
}

// Commands that are universally implemented
var networkcheckCommands = []*cobra.Command{
	_networkinspectCommand,
	_networklistCommand,
	_networkrmCommand,
}

func init() {
	networkcheckCommand.AddCommand(networkcheckCommands...)
	networkcheckCommand.SetUsageTemplate(UsageTemplate())
	rootCmd.AddCommand(networkcheckCommand.Command)
}
