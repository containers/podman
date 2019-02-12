package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var (
	trustCommand = cliconfig.PodmanCommand{
		Command: &cobra.Command{
			Use:   "trust",
			Short: "Manage container image trust policy",
			Long:  "podman image trust command",
		},
	}
)

func init() {
	trustCommand.SetUsageTemplate(UsageTemplate())
	trustCommand.AddCommand(getTrustSubCommands()...)
	imageCommand.AddCommand(trustCommand.Command)
}
