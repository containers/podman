// +build !remoteclient

package main

import (
	"errors"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/spf13/cobra"
)

var (
	networklistCommand     cliconfig.NetworkListValues
	networklistDescription = `List networks`
	_networklistCommand    = &cobra.Command{
		Use:   "ls",
		Args:  noSubArgs,
		Short: "network list",
		Long:  networklistDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			networklistCommand.InputArgs = args
			networklistCommand.GlobalFlags = MainGlobalOpts
			networklistCommand.Remote = remoteclient
			return networklistCmd(&networklistCommand)
		},
		Example: `podman network list`,
	}
)

func init() {
	networklistCommand.Command = _networklistCommand
	networklistCommand.SetHelpTemplate(HelpTemplate())
	networklistCommand.SetUsageTemplate(UsageTemplate())
	flags := networklistCommand.Flags()
	// TODO enable filters based on something
	//flags.StringSliceVarP(&networklistCommand.Filter, "filter", "f",  []string{}, "Pause all running containers")
	flags.BoolVarP(&networklistCommand.Quiet, "quiet", "q", false, "display only names")
}

func networklistCmd(c *cliconfig.NetworkListValues) error {
	if rootless.IsRootless() && !remoteclient {
		return errors.New("network list is not supported for rootless mode")
	}
	if len(c.InputArgs) > 0 {
		return errors.New("network list takes no arguments")
	}
	runtime, err := adapter.GetRuntimeNoStore(getContext(), &c.PodmanCommand)
	if err != nil {
		return err
	}
	return runtime.NetworkList(c)
}
