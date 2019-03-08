package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var healthcheckDescription = "Manage health checks on containers"
var healthcheckCommand = cliconfig.PodmanCommand{
	Command: &cobra.Command{
		Use:   "healthcheck",
		Short: "Manage Healthcheck",
		Long:  healthcheckDescription,
		RunE:  commandRunE(),
	},
}

// Commands that are universally implemented
var healthcheckCommands []*cobra.Command

func init() {
	healthcheckCommand.AddCommand(healthcheckCommands...)
	healthcheckCommand.AddCommand(getHealtcheckSubCommands()...)
	healthcheckCommand.SetUsageTemplate(UsageTemplate())
	rootCmd.AddCommand(healthcheckCommand.Command)
}
