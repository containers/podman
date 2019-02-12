package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var (
	podDescription = `Manage container pods.

Pods are a group of one or more containers sharing the same network, pid and ipc namespaces.`
)
var podCommand = cliconfig.PodmanCommand{
	Command: &cobra.Command{
		Use:   "pod",
		Short: "Manage pods",
		Long:  podDescription,
	},
}

func init() {
	podCommand.AddCommand(getPodSubCommands()...)
	podCommand.SetUsageTemplate(UsageTemplate())
}
