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
		},
	}
)

func init() {
	systemCommand.AddCommand(getSystemSubCommands()...)
	rootCmd.AddCommand(systemCommand.Command)
}
