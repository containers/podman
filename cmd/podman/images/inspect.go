package images

import (
	"github.com/containers/podman/v2/cmd/podman/inspect"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman image _inspect_
	inspectCmd = &cobra.Command{
		Use:   "inspect [options] IMAGE [IMAGE...]",
		Short: "Display the configuration of an image",
		Long:  `Displays the low-level information of an image identified by name or ID.`,
		RunE:  inspectExec,
		Example: `podman inspect alpine
  podman inspect --format "imageId: {{.Id}} size: {{.Size}}" alpine
  podman inspect --format "image: {{.ImageName}} driver: {{.Driver}}" myctr`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  imageCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := inspectCmd.Flags()
	flags.StringVarP(&inspectOpts.Format, "format", "f", "json", "Format the output to a Go template or json")
}

func inspectExec(cmd *cobra.Command, args []string) error {
	inspectOpts.Type = inspect.ImageType
	return inspect.Inspect(args, *inspectOpts)
}
