package containers

import (
	"context"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	containerExistsDescription = `If the named container exists in local storage, podman container exists exits with 0, otherwise the exit code will be 1.`

	existsCommand = &cobra.Command{
		Use:   "exists CONTAINER",
		Short: "Check if a container exists in local storage",
		Long:  containerExistsDescription,
		Example: `podman container exists containerID
  podman container exists myctr || podman run --name myctr [etc...]`,
		RunE:                  exists,
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: existsCommand,
		Parent:  containerCmd,
	})
}

func exists(cmd *cobra.Command, args []string) error {
	response, err := registry.ContainerEngine().ContainerExists(context.Background(), args[0])
	if err != nil {
		return err
	}
	if !response.Value {
		registry.SetExitCode(1)
	}
	return nil
}
