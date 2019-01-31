package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var playCommand cliconfig.PodmanCommand

func init() {
	var playDescription = "Play a pod and its containers from a structured file."
	playCommand.Command = &cobra.Command{
		Use:   "play",
		Short: "Play a pod",
		Long:  playDescription,
	}

}

func init() {
	playCommand.AddCommand(getPlaySubCommands()...)
	rootCmd.AddCommand(playCommand.Command)
}
