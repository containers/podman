package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var manifestDescription = "Creates, modifies, and pushes manifest lists and image indexes"
var manifestCommand = cliconfig.PodmanCommand{
	Command: &cobra.Command{
		Use:   "manifest",
		Short: "Manipulate manifest lists and image indexes",
		Long:  manifestDescription,
		RunE:  commandRunE(),
		Example: `podman manifest create localhost/list
  podman manifest add localhost/list localhost/image
  podman manifest annotate --annotation A=B localhost/list localhost/image
  podman manifest annotate --annotation A=B localhost/list sha256:entryManifestDigest
  podman manifest remove localhost/list sha256:entryManifestDigest
  podman manifest inspect localhost/list
  podman manifest push localhost/list transport:destination`,
	},
}

var manifestCommands = []*cobra.Command{
	_manifestAddCommand,
	_manifestAnnotateCommand,
	_manifestCreateCommand,
	_manifestInspectCommand,
	_manifestPushCommand,
	_manifestRemoveCommand,
}

func init() {
	manifestCommand.AddCommand(manifestCommands...)
	manifestCommand.SetUsageTemplate(UsageTemplate())
	rootCmd.AddCommand(manifestCommand.Command)
}
