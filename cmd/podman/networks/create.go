package network

import (
	"fmt"
	"net"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/network"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networkCreateDescription = `create CNI networks for containers and pods`
	networkCreateCommand     = &cobra.Command{
		Use:     "create [flags] [NETWORK]",
		Short:   "network create",
		Long:    networkCreateDescription,
		RunE:    networkCreate,
		Example: `podman network create podman1`,
		Annotations: map[string]string{
			registry.ParentNSRequired: "",
		},
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
	//flags.StringVar(&networkCreateOptions.IPamDriver, "ipam-driver", "",  "IP Address Management Driver")
	// TODO enable when IPv6 is working
	//flags.BoolVar(&networkCreateOptions.IPV6, "IPv6", false, "enable IPv6 networking")
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
	if err := network.IsSupportedDriver(networkCreateOptions.Driver); err != nil {
		return err
	}
	if len(args) > 1 {
		return errors.Errorf("only one network can be created at a time")
	}
	if len(args) > 0 && !libpod.NameRegex.MatchString(args[0]) {
		return libpod.RegexError
	}

	if len(args) > 0 {
		name = args[0]
	}
	response, err := registry.ContainerEngine().NetworkCreate(registry.Context(), name, networkCreateOptions)
	if err != nil {
		return err
	}
	fmt.Println(response.Filename)
	return nil
}
