package main

import (
	"github.com/containers/podman/v2/cmd/podman/inspect"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _inspect_ Object_ID
	inspectCmd = &cobra.Command{
		Use:   "inspect [flags] {CONTAINER_ID | IMAGE_ID}",
		Short: "Display the configuration of object denoted by ID",
		Long:  "Displays the low-level information on an object identified by name or ID",
		RunE:  inspectExec,
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
