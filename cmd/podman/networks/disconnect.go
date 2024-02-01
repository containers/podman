package network

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networkDisconnectDescription = `Remove container from a network`
	networkDisconnectCommand     = &cobra.Command{
		Use:               "disconnect [options] NETWORK CONTAINER",
		Short:             "Disconnect a container from a network",
		Long:              networkDisconnectDescription,
		RunE:              networkDisconnect,
		Example:           `podman network disconnect web secondary`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteNetworkConnectCmd,
	}
)

var (
	networkDisconnectOptions entities.NetworkDisconnectOptions
)

func networkDisconnectFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&networkDisconnectOptions.Force, "force", "f", false, "force removal of container from network")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkDisconnectCommand,
		Parent:  networkCmd,
	})
	flags := networkDisconnectCommand.Flags()
	networkDisconnectFlags(flags)
}

func networkDisconnect(cmd *cobra.Command, args []string) error {
	networkDisconnectOptions.Container = args[1]
	return registry.ContainerEngine().NetworkDisconnect(registry.Context(), args[0], networkDisconnectOptions)
}
