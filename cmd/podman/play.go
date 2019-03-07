package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var (
	playCommand     cliconfig.PodmanCommand
	playDescription = "Play a pod and its containers from a structured file."
	_playCommand    = &cobra.Command{
		Use:   "play",
		Short: "Play a pod",
		Long:  playDescription,
	}
)

func init() {
	playCommand.Command = _playCommand
	playCommand.SetHelpTemplate(HelpTemplate())
	playCommand.SetUsageTemplate(UsageTemplate())
	playCommand.AddCommand(getPlaySubCommands()...)
}
