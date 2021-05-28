package network

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	networkConnectDescription = `Add container to a network`
	networkConnectCommand     = &cobra.Command{
		Use:               "connect [options] NETWORK CONTAINER",
		Short:             "network connect",
		Long:              networkConnectDescription,
		RunE:              networkConnect,
		Example:           `podman network connect web secondary`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteNetworkConnectCmd,
	}
)

var (
	networkConnectOptions entities.NetworkConnectOptions
)

func networkConnectFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	aliasFlagName := "alias"
	flags.StringSliceVar(&networkConnectOptions.Aliases, aliasFlagName, []string{}, "network scoped alias for container")
	_ = cmd.RegisterFlagCompletionFunc(aliasFlagName, completion.AutocompleteNone)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkConnectCommand,
		Parent:  networkCmd,
	})
	networkConnectFlags(networkConnectCommand)
}

func networkConnect(cmd *cobra.Command, args []string) error {
	networkConnectOptions.Container = args[1]
	return registry.ContainerEngine().NetworkConnect(registry.Context(), args[0], networkConnectOptions)
}
