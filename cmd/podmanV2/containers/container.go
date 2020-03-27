package containers

import (
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _container_
	containerCmd = &cobra.Command{
		Use:               "container",
		Short:             "Manage containers",
		Long:              "Manage containers",
		TraverseChildren:  true,
		PersistentPreRunE: preRunE,
		RunE:              registry.SubCommandExists,
	}

	defaultContainerConfig = getDefaultContainerConfig()
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerCmd,
	})
	containerCmd.SetHelpTemplate(registry.HelpTemplate())
	containerCmd.SetUsageTemplate(registry.UsageTemplate())
}

func preRunE(cmd *cobra.Command, args []string) error {
	_, err := registry.NewContainerEngine(cmd, args)
	return err
}

func getDefaultContainerConfig() *config.Config {
	defaultContainerConfig, err := config.Default()
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	return defaultContainerConfig
}
