package containers

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/inspect"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// podman container _inspect_
	containerInspectCmd = &cobra.Command{
		Use:               "inspect [options] CONTAINER [CONTAINER...]",
		Short:             "Display the configuration of a container",
		Long:              `Displays the low-level information on a container identified by name or ID.`,
		RunE:              inspectExec,
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman container inspect myCtr
  podman container inspect -l --format '{{.Id}} {{.Config.Labels}}'`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	if !registry.IsRemote() {
		containerInspectCmd.Example += "\n  podman container inspect --latest"
	}
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerInspectCmd,
		Parent:  containerCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := containerInspectCmd.Flags()
	flags.BoolVarP(&inspectOpts.Size, "size", "s", false, "Display total file size")

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = containerInspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&define.InspectContainerData{}))

	validate.AddLatestFlag(containerInspectCmd, &inspectOpts.Latest)
}

func inspectExec(cmd *cobra.Command, args []string) error {
	// Force container type
	inspectOpts.Type = common.ContainerType
	return inspect.Inspect(args, *inspectOpts)
}
