package containers

import (
	"github.com/containers/libpod/cmd/podman/inspect"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// podman container _inspect_
	inspectCmd = &cobra.Command{
		Use:   "inspect [flags] CONTAINER",
		Short: "Display the configuration of a container",
		Long:  `Displays the low-level information on a container identified by name or ID.`,
		RunE:  inspectExec,
		Example: `podman container inspect myCtr
  podman container inspect -l --format '{{.Id}} {{.Config.Labels}}'`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  containerCmd,
	})
	inspectOpts = inspect.AddInspectFlagSet(inspectCmd)
}

func inspectExec(cmd *cobra.Command, args []string) error {
	return inspect.Inspect(args, *inspectOpts)
}
