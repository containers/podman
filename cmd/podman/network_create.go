// +build !remoteclient

package main

import (
	"fmt"
	"github.com/containers/libpod/pkg/network"
	"net"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	networkCreateCommand     cliconfig.NetworkCreateValues
	networkCreateDescription = `create CNI networks for containers and pods`
	_networkCreateCommand    = &cobra.Command{
		Use:   "create [flags] [NETWORK]",
		Short: "network create",
		Long:  networkCreateDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			networkCreateCommand.InputArgs = args
			networkCreateCommand.GlobalFlags = MainGlobalOpts
			networkCreateCommand.Remote = remoteclient
			return networkcreateCmd(&networkCreateCommand)
		},
		Example: `podman network create podman1`,
	}
)

func init() {
	networkCreateCommand.Command = _networkCreateCommand
	networkCreateCommand.SetHelpTemplate(HelpTemplate())
	networkCreateCommand.SetUsageTemplate(UsageTemplate())
	flags := networkCreateCommand.Flags()
	flags.StringVarP(&networkCreateCommand.Driver, "driver", "d", "bridge", "driver to manage the network")
	flags.IPVar(&networkCreateCommand.Gateway, "gateway", nil, "IPv4 or IPv6 gateway for the subnet")
	flags.BoolVar(&networkCreateCommand.Internal, "internal", false, "restrict external access from this network")
	flags.IPNetVar(&networkCreateCommand.IPRange, "ip-range", net.IPNet{}, "allocate container IP from range")
	// TODO not supported yet
	//flags.StringVar(&networkCreateCommand.IPamDriver, "ipam-driver", "",  "IP Address Management Driver")
	// TODO enable when IPv6 is working
	//flags.BoolVar(&networkCreateCommand.IPV6, "IPv6", false, "enable IPv6 networking")
	flags.IPNetVar(&networkCreateCommand.Network, "subnet", net.IPNet{}, "subnet in CIDR format")

}

func networkcreateCmd(c *cliconfig.NetworkCreateValues) error {
	if err := network.IsSupportedDriver(c.Driver); err != nil {
		return err
	}
	if rootless.IsRootless() && !remoteclient {
		return errors.New("network create is not supported for rootless mode")
	}
	if len(c.InputArgs) > 1 {
		return errors.Errorf("only one network can be created at a time")
	}
	runtime, err := adapter.GetRuntimeNoStore(getContext(), &c.PodmanCommand)
	if err != nil {
		return err
	}
	fileName, err := runtime.NetworkCreate(c)
	if err == nil {
		fmt.Println(fileName)
	}
	return err
}
