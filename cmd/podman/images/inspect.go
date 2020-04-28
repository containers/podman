package images

import (
	"github.com/containers/libpod/cmd/podman/inspect"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman image _inspect_
	inspectCmd = &cobra.Command{
		Use:   "inspect [flags] IMAGE",
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
	inspectOpts = inspect.AddInspectFlagSet(inspectCmd)
	flags := inspectCmd.Flags()
	_ = flags.MarkHidden("latest") // Shared with container-inspect but not wanted here.
}

func inspectExec(cmd *cobra.Command, args []string) error {
	return inspect.Inspect(args, *inspectOpts)
}
