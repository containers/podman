package containers

import (
	"os"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _container_
	containerCmd = &cobra.Command{
		Use:              "container",
		Short:            "Manage containers",
		Long:             "Manage containers",
		TraverseChildren: true,
		RunE:             registry.SubCommandExists,
	}

	defaultContainerConfig = getDefaultContainerConfig()
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerCmd,
	})
}

func getDefaultContainerConfig() *config.Config {
	defaultContainerConfig, err := config.Default()
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	return defaultContainerConfig
}
