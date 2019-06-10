package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var (
	trustDescription = `Manages which registries you trust as a source of container images based on its location.

  The location is determined by the transport and the registry host of the image.  Using this container image docker://docker.io/library/busybox as an example, docker is the transport and docker.io is the registry host.`
	trustCommand = cliconfig.PodmanCommand{
		Command: &cobra.Command{
			Use:   "trust",
			Short: "Manage container image trust policy",
			Long:  trustDescription,
			RunE:  commandRunE(),
		},
	}
)

func init() {
	trustCommand.SetHelpTemplate(HelpTemplate())
	trustCommand.SetUsageTemplate(UsageTemplate())
	trustCommand.AddCommand(getTrustSubCommands()...)
	imageCommand.AddCommand(trustCommand.Command)
}
