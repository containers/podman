package volumes

import (
	"github.com/containers/podman/v2/cmd/podman/inspect"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	volumeInspectDescription = `Display detailed information on one or more volumes.

  Use a Go template to change the format from JSON.`
	inspectCommand = &cobra.Command{
		Use:   "inspect [options] VOLUME [VOLUME...]",
		Short: "Display detailed information on one or more volumes",
		Long:  volumeInspectDescription,
		RunE:  volumeInspect,
		Example: `podman volume inspect myvol
  podman volume inspect --all
  podman volume inspect --format "{{.Driver}} {{.Scope}}" myvol`,
	}
)

var (
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCommand,
		Parent:  volumeCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := inspectCommand.Flags()
	flags.BoolVarP(&inspectOpts.All, "all", "a", false, "Inspect all volumes")
	flags.StringVarP(&inspectOpts.Format, "format", "f", "json", "Format volume output using Go template")
}

func volumeInspect(cmd *cobra.Command, args []string) error {
	if (inspectOpts.All && len(args) > 0) || (!inspectOpts.All && len(args) < 1) {
		return errors.New("provide one or more volume names or use --all")
	}
	inspectOpts.Type = inspect.VolumeType
	return inspect.Inspect(args, *inspectOpts)
}
