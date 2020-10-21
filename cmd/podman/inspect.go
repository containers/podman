package main

import (
	"github.com/containers/podman/v2/cmd/podman/inspect"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	inspectDescription = `Displays the low-level information on an object identified by name or ID.
  For more inspection options, see:

      podman container inspect
      podman image inspect
      podman network inspect
      podman pod inspect
      podman volume inspect`

	// Command: podman _inspect_ Object_ID
	inspectCmd = &cobra.Command{
		Use:              "inspect [options] {CONTAINER_ID | IMAGE_ID} [...]",
		Short:            "Display the configuration of object denoted by ID",
		RunE:             inspectExec,
		Long:             inspectDescription,
		TraverseChildren: true,
		Example: `podman inspect fedora
  podman inspect --type image fedora
  podman inspect CtrID ImgID
  podman inspect --format "imageId: {{.Id}} size: {{.Size}}" fedora`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
	})
	inspectOpts = inspect.AddInspectFlagSet(inspectCmd)
}

func inspectExec(cmd *cobra.Command, args []string) error {
	return inspect.Inspect(args, *inspectOpts)
}
