package network

import (
	"fmt"
	"net"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networkCreateDescription = `create CNI networks for containers and pods`
	networkCreateCommand     = &cobra.Command{
		Use:     "create [options] [NETWORK]",
		Short:   "network create",
		Long:    networkCreateDescription,
		RunE:    networkCreate,
		Args:    cobra.MaximumNArgs(1),
		Example: `podman network create podman1`,
	}
)

var (
	networkCreateOptions entities.NetworkCreateOptions
)

func networkCreateFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&networkCreateOptions.Driver, "driver", "d", "bridge", "driver to manage the network")
	flags.IPVar(&networkCreateOptions.Gateway, "gateway", nil, "IPv4 or IPv6 gateway for the subnet")
	flags.BoolVar(&networkCreateOptions.Internal, "internal", false, "restrict external access from this network")
	flags.IPNetVar(&networkCreateOptions.Range, "ip-range", net.IPNet{}, "allocate container IP from range")
	flags.StringVar(&networkCreateOptions.MacVLAN, "macvlan", "", "create a Macvlan connection based on this device")
	// TODO not supported yet
	// flags.StringVar(&networkCreateOptions.IPamDriver, "ipam-driver", "",  "IP Address Management Driver")
	// TODO enable when IPv6 is working
	// flags.BoolVar(&networkCreateOptions.IPV6, "IPv6", false, "enable IPv6 networking")
	flags.IPNetVar(&networkCreateOptions.Subnet, "subnet", net.IPNet{}, "subnet in CIDR format")
	flags.BoolVar(&networkCreateOptions.DisableDNS, "disable-dns", false, "disable dns plugin")
}
func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: networkCreateCommand,
		Parent:  networkCmd,
	})
	flags := networkCreateCommand.Flags()
	networkCreateFlags(flags)

}

func networkCreate(cmd *cobra.Command, args []string) error {
	var (
		name string
	)
	if len(args) > 0 {
		if !define.NameRegex.MatchString(args[0]) {
			return define.RegexError
		}
		name = args[0]
	}
	response, err := registry.ContainerEngine().NetworkCreate(registry.Context(), name, networkCreateOptions)
	if err != nil {
		return err
	}
	fmt.Println(response.Filename)
	return nil
}
