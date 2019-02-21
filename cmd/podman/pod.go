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

//podSubCommands are implemented both in local and remote clients
var podSubCommands = []*cobra.Command{
	_podExistsCommand,
	_podInspectCommand,
	_podRmCommand,
}

func init() {
	podCommand.AddCommand(podSubCommands...)
	podCommand.AddCommand(getPodSubCommands()...)
	podCommand.SetUsageTemplate(UsageTemplate())
}
