package containers

import (
	"context"
	"os"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	containerExistsCommand = &cobra.Command{
		Use:   "exists CONTAINER",
		Short: "Check if a container exists in local storage",
		Long:  containerExistsDescription,
		Example: `podman container exists containerID
  podman container exists myctr || podman run --name myctr [etc...]`,
		RunE: exists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerExistsCommand,
		Parent:  containerCmd,
	})
}

func exists(cmd *cobra.Command, args []string) error {
	exists, err := registry.ContainerEngine().ContainerExists(context.Background(), args[0])
	if err != nil {
		return err
	}
	if !exists {
		os.Exit(1)
	}
	return nil
}
