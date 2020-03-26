package images

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _image_
	imageCmd = &cobra.Command{
		Use:               "image",
		Short:             "Manage images",
		Long:              "Manage images",
		TraverseChildren:  true,
		PersistentPreRunE: preRunE,
		RunE:              registry.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imageCmd,
	})
	imageCmd.SetHelpTemplate(registry.HelpTemplate())
	imageCmd.SetUsageTemplate(registry.UsageTemplate())
}

func preRunE(cmd *cobra.Command, args []string) error {
	if _, err := registry.NewImageEngine(cmd, args); err != nil {
		return err
	}
	return nil
}
