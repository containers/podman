package network

import (
	"github.com/containers/podman/v2/cmd/podman/inspect"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	networkinspectDescription = `Inspect network`
	networkinspectCommand     = &cobra.Command{
		Use:     "inspect [options] NETWORK [NETWORK...]",
		Short:   "network inspect",
		Long:    networkinspectDescription,
		RunE:    networkInspect,
		Example: `podman network inspect podman`,
		Args:    cobra.MinimumNArgs(1),
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: networkinspectCommand,
		Parent:  networkCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := networkinspectCommand.Flags()
	flags.StringVarP(&inspectOpts.Format, "format", "f", "", "Pretty-print network to JSON or using a Go template")
}

func networkInspect(_ *cobra.Command, args []string) error {
	inspectOpts.Type = inspect.NetworkType
	return inspect.Inspect(args, *inspectOpts)

}
