package images

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _network_
	cmd = &cobra.Command{
		Use:               "network",
		Short:             "Manage networks",
		Long:              "Manage networks",
		TraverseChildren:  true,
		PersistentPreRunE: preRunE,
		RunE:              registry.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: cmd,
	})
	cmd.SetHelpTemplate(registry.HelpTemplate())
	cmd.SetUsageTemplate(registry.UsageTemplate())
}

func preRunE(cmd *cobra.Command, args []string) error {
	_, err := registry.NewContainerEngine(cmd, args)
	return err
}
