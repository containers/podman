package network

import (
	"fmt"
	"net"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
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
	flags.StringVarP(&networkCreateOptions.Driver, driverFlagName, "d", "bridge", "driver to manage the network")
	_ = cmd.RegisterFlagCompletionFunc(driverFlagName, common.AutocompleteNetworkDriver)

	optFlagName := "opt"
	flags.StringArrayVarP(&opts, optFlagName, "o", []string{}, "Set driver specific options (default [])")
	_ = cmd.RegisterFlagCompletionFunc(optFlagName, completion.AutocompleteNone)

	gatewayFlagName := "gateway"
	flags.IPVar(&networkCreateOptions.Gateway, gatewayFlagName, nil, "IPv4 or IPv6 gateway for the subnet")
	_ = cmd.RegisterFlagCompletionFunc(gatewayFlagName, completion.AutocompleteNone)

	flags.BoolVar(&networkCreateOptions.Internal, "internal", false, "restrict external access from this network")

	ipRangeFlagName := "ip-range"
	flags.IPNetVar(&networkCreateOptions.Range, ipRangeFlagName, net.IPNet{}, "allocate container IP from range")
	_ = cmd.RegisterFlagCompletionFunc(ipRangeFlagName, completion.AutocompleteNone)

	macvlanFlagName := "macvlan"
	flags.StringVar(&networkCreateOptions.MacVLAN, macvlanFlagName, "", "create a Macvlan connection based on this device")
	_ = cmd.RegisterFlagCompletionFunc(macvlanFlagName, completion.AutocompleteNone)

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
		if !define.NameRegex.MatchString(args[0]) {
			return define.RegexError
		}
		name = args[0]
	}
	var err error
	networkCreateOptions.Labels, err = parse.GetAllLabels([]string{}, labels)
	if err != nil {
		return errors.Wrap(err, "failed to parse labels")
	}
	networkCreateOptions.Options, err = parse.GetAllLabels([]string{}, opts)
	if err != nil {
		return errors.Wrapf(err, "unable to process options")
	}
	response, err := registry.ContainerEngine().NetworkCreate(registry.Context(), name, networkCreateOptions)
	if err != nil {
		return err
	}
	fmt.Println(response.Filename)
	return nil
}
