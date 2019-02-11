package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var volumeDescription = `Manage volumes.

Volumes are created in and can be shared between containers.`

var volumeCommand = cliconfig.PodmanCommand{
	Command: &cobra.Command{
		Use:   "volume",
		Short: "Manage volumes",
		Long:  volumeDescription,
	},
}

func init() {
	volumeCommand.AddCommand(getVolumeSubCommands()...)
}
