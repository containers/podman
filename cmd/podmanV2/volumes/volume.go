package volumes

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _volume_
	volumeCmd = &cobra.Command{
		Use:               "volume",
		Short:             "Manage volumes",
		Long:              "Volumes are created in and can be shared between containers",
		TraverseChildren:  true,
		PersistentPreRunE: preRunE,
		RunE:              registry.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: volumeCmd,
	})
	volumeCmd.SetHelpTemplate(registry.HelpTemplate())
	volumeCmd.SetUsageTemplate(registry.UsageTemplate())
}

func preRunE(cmd *cobra.Command, args []string) error {
	_, err := registry.NewContainerEngine(cmd, args)
	return err
}
