package healthcheck

import (
	"context"
	"fmt"

	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	healthcheckRunDescription = "run the health check of a container"
	healthcheckrunCommand     = &cobra.Command{
		Use:                   "run CONTAINER",
		Short:                 "run the health check of a container",
		Long:                  healthcheckRunDescription,
		Example:               `podman healthcheck run mywebapp`,
		RunE:                  run,
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     common.AutocompleteContainersRunning,
		DisableFlagsInUseLine: true,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: healthcheckrunCommand,
		Parent:  healthCmd,
	})
}

func run(cmd *cobra.Command, args []string) error {
	response, err := registry.ContainerEngine().HealthCheckRun(context.Background(), args[0], entities.HealthCheckOptions{})
	if err != nil {
		return err
	}
	if response.Status == "unhealthy" {
		registry.SetExitCode(1)
	}
	fmt.Println(response.Status)
	return err
}
