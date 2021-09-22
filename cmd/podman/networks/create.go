package network

import (
	"fmt"
	"net"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/libpod/network/util"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	networkCreateDescription = `create CNI networks for containers and pods`
	networkCreateCommand     = &cobra.Command{
		Use:               "create [options] [NAME]",
		Short:             "network create",
		Long:              networkCreateDescription,
		RunE:              networkCreate,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman network create podman1`,
	}
)

var (
	networkCreateOptions entities.NetworkCreateOptions
	labels               []string
	opts                 []string
)

func networkCreateFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	driverFlagName := "driver"
	flags.StringVarP(&networkCreateOptions.Driver, driverFlagName, "d", types.DefaultNetworkDriver, "driver to manage the network")
	_ = cmd.RegisterFlagCompletionFunc(driverFlagName, common.AutocompleteNetworkDriver)

	optFlagName := "opt"
	flags.StringArrayVarP(&opts, optFlagName, "o", nil, "Set driver specific options (default [])")
	_ = cmd.RegisterFlagCompletionFunc(optFlagName, completion.AutocompleteNone)

	gatewayFlagName := "gateway"
	flags.IPVar(&networkCreateOptions.Gateway, gatewayFlagName, nil, "IPv4 or IPv6 gateway for the subnet")
	_ = cmd.RegisterFlagCompletionFunc(gatewayFlagName, completion.AutocompleteNone)

	flags.BoolVar(&networkCreateOptions.Internal, "internal", false, "restrict external access from this network")

	ipRangeFlagName := "ip-range"
	flags.IPNetVar(&networkCreateOptions.Range, ipRangeFlagName, net.IPNet{}, "allocate container IP from range")
	_ = cmd.RegisterFlagCompletionFunc(ipRangeFlagName, completion.AutocompleteNone)

	// TODO consider removing this for 4.0
	macvlanFlagName := "macvlan"
	flags.StringVar(&networkCreateOptions.MacVLAN, macvlanFlagName, "", "create a Macvlan connection based on this device")
	// This option is deprecated
	flags.MarkHidden(macvlanFlagName)

	labelFlagName := "label"
	flags.StringArrayVar(&labels, labelFlagName, nil, "set metadata on a network")
	_ = cmd.RegisterFlagCompletionFunc(labelFlagName, completion.AutocompleteNone)

	// TODO not supported yet
	// flags.StringVar(&networkCreateOptions.IPamDriver, "ipam-driver", "",  "IP Address Management Driver")

	flags.BoolVar(&networkCreateOptions.IPv6, "ipv6", false, "enable IPv6 networking")

	subnetFlagName := "subnet"
	flags.IPNetVar(&networkCreateOptions.Subnet, subnetFlagName, net.IPNet{}, "subnet in CIDR format")
	_ = cmd.RegisterFlagCompletionFunc(subnetFlagName, completion.AutocompleteNone)

	flags.BoolVar(&networkCreateOptions.DisableDNS, "disable-dns", false, "disable dns plugin")
}
func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkCreateCommand,
		Parent:  networkCmd,
	})
	networkCreateFlags(networkCreateCommand)
}

func networkCreate(cmd *cobra.Command, args []string) error {
	var (
		name string
	)
	if len(args) > 0 {
		name = args[0]
	}
	var err error
	networkCreateOptions.Labels, err = parse.GetAllLabels([]string{}, labels)
	if err != nil {
		return errors.Wrap(err, "failed to parse labels")
	}
	networkCreateOptions.Options, err = parse.GetAllLabels([]string{}, opts)
	if err != nil {
		return errors.Wrapf(err, "unable to parse options")
	}

	network := types.Network{
		Name:        name,
		Driver:      networkCreateOptions.Driver,
		Options:     networkCreateOptions.Options,
		Labels:      networkCreateOptions.Labels,
		IPv6Enabled: networkCreateOptions.IPv6,
		DNSEnabled:  !networkCreateOptions.DisableDNS,
		Internal:    networkCreateOptions.Internal,
	}

	// old --macvlan option
	if networkCreateOptions.MacVLAN != "" {
		logrus.Warn("The --macvlan option is deprecated, use `--driver macvlan --opt parent=<device>` instead")
		network.Driver = types.MacVLANNetworkDriver
		network.NetworkInterface = networkCreateOptions.MacVLAN
	} else if networkCreateOptions.Driver == types.MacVLANNetworkDriver {
		// new -d macvlan --opt parent=... syntax
		if parent, ok := network.Options["parent"]; ok {
			network.NetworkInterface = parent
			delete(network.Options, "parent")
		}
	}

	if networkCreateOptions.Subnet.IP != nil {
		s := types.Subnet{
			Subnet:  types.IPNet{IPNet: networkCreateOptions.Subnet},
			Gateway: networkCreateOptions.Gateway,
		}
		if networkCreateOptions.Range.IP != nil {
			startIP, err := util.FirstIPInSubnet(&networkCreateOptions.Range)
			if err != nil {
				return errors.Wrap(err, "failed to get first ip in range")
			}
			lastIP, err := util.LastIPInSubnet(&networkCreateOptions.Range)
			if err != nil {
				return errors.Wrap(err, "failed to get last ip in range")
			}
			s.LeaseRange = &types.LeaseRange{
				StartIP: startIP,
				EndIP:   lastIP,
			}
		}
		network.Subnets = append(network.Subnets, s)
	} else if networkCreateOptions.Range.IP != nil || networkCreateOptions.Gateway != nil {
		return errors.New("cannot set gateway or range without subnet")
	}

	response, err := registry.ContainerEngine().NetworkCreate(registry.Context(), network)
	if err != nil {
		return err
	}
	fmt.Println(response.Name)
	return nil
}
