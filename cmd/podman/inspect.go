package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/common"
	"github.com/containers/libpod/cmd/podman/containers"
	"github.com/containers/libpod/cmd/podman/images"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// Inspect is one of the outlier commands in that it operates on images/containers/...

var (
	inspectOpts *entities.InspectOptions

	// Command: podman _inspect_ Object_ID
	inspectCmd = &cobra.Command{
		Use:              "inspect [flags] {CONTAINER_ID | IMAGE_ID}",
		Short:            "Display the configuration of object denoted by ID",
		Long:             "Displays the low-level information on an object identified by name or ID",
		TraverseChildren: true,
		RunE:             inspect,
		Example: `podman inspect alpine
  podman inspect --format "imageId: {{.Id}} size: {{.Size}}" alpine`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
	})
	inspectOpts = common.AddInspectFlagSet(inspectCmd)
	flags := inspectCmd.Flags()
	flags.StringVarP(&inspectOpts.Type, "type", "t", "", "Return JSON for specified type, (image or container) (default \"all\")")
	if !registry.IsRemote() {
		flags.BoolVarP(&inspectOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of (containers only)")
	}
}

func inspect(cmd *cobra.Command, args []string) error {
	switch inspectOpts.Type {
	case "image":
		return images.Inspect(cmd, args, inspectOpts)
	case "container":
		return containers.Inspect(cmd, args, inspectOpts)
	case "":
		if err := images.Inspect(cmd, args, inspectOpts); err == nil {
			return nil
		}
		return containers.Inspect(cmd, args, inspectOpts)
	default:
		return fmt.Errorf("invalid type %q is must be 'container' or 'image'", inspectOpts.Type)
	}
}
