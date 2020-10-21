package containers

import (
	"github.com/containers/podman/v2/cmd/podman/inspect"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// podman container _inspect_
	inspectCmd = &cobra.Command{
		Use:   "inspect [options] CONTAINER [CONTAINER...]",
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
	inspectOpts = new(entities.InspectOptions)
	flags := inspectCmd.Flags()
	flags.BoolVarP(&inspectOpts.Size, "size", "s", false, "Display total file size")
	flags.StringVarP(&inspectOpts.Format, "format", "f", "json", "Format the output to a Go template or json")
	validate.AddLatestFlag(inspectCmd, &inspectOpts.Latest)
}

func inspectExec(cmd *cobra.Command, args []string) error {
	// Force container type
	inspectOpts.Type = inspect.ContainerType
	return inspect.Inspect(args, *inspectOpts)
}
