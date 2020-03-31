package healthcheck

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: healthcheck
	healthCmd = &cobra.Command{
		Use:               "healthcheck",
		Short:             "Manage Healthcheck",
		Long:              "Manage Healthcheck",
		TraverseChildren:  true,
		PersistentPreRunE: preRunE,
		RunE:              registry.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: healthCmd,
	})
	healthCmd.SetHelpTemplate(registry.HelpTemplate())
	healthCmd.SetUsageTemplate(registry.UsageTemplate())
}

func preRunE(cmd *cobra.Command, args []string) error {
	_, err := registry.NewContainerEngine(cmd, args)
	return err
}
