// +build !remoteclient

package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	networkrmCommand     cliconfig.NetworkRmValues
	networkrmDescription = `Remove networks`
	_networkrmCommand    = &cobra.Command{
		Use:   "rm [flags] NETWORK [NETWORK...]",
		Short: "network rm",
		Long:  networkrmDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			networkrmCommand.InputArgs = args
			networkrmCommand.GlobalFlags = MainGlobalOpts
			networkrmCommand.Remote = remoteclient
			return networkrmCmd(&networkrmCommand)
		},
		Example: `podman network rm podman`,
	}
)

func init() {
	networkrmCommand.Command = _networkrmCommand
	networkrmCommand.SetHelpTemplate(HelpTemplate())
	networkrmCommand.SetUsageTemplate(UsageTemplate())
}

func networkrmCmd(c *cliconfig.NetworkRmValues) error {
	if rootless.IsRootless() && !remoteclient {
		return errors.New("network rm is not supported for rootless mode")
	}
	if len(c.InputArgs) < 1 {
		return errors.Errorf("at least one network name is required")
	}
	runtime, err := adapter.GetRuntimeNoStore(getContext(), &c.PodmanCommand)
	if err != nil {
		return err
	}
	return runtime.NetworkRemove(c)
}
