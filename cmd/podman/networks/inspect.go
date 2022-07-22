package network

import (
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/inspect"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	networkinspectDescription = `Inspect network`
	networkinspectCommand     = &cobra.Command{
		Use:               "inspect [options] NETWORK [NETWORK...]",
		Long:              "Displays the network configuration for one or more networks.",
		Short:             networkinspectDescription,
		RunE:              networkInspect,
		Example:           `podman network inspect podman`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteNetworks,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkinspectCommand,
		Parent:  networkCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := networkinspectCommand.Flags()

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "", "Pretty-print network to JSON or using a Go template")
	_ = networkinspectCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&types.Network{}))
}

func networkInspect(_ *cobra.Command, args []string) error {
	inspectOpts.Type = common.NetworkType
	return inspect.Inspect(args, *inspectOpts)
}
