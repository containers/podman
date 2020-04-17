package main

import (
	"context"
	"fmt"

	"github.com/containers/image/v5/docker/reference"
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
		Args:             cobra.ExactArgs(1),
		Short:            "Display the configuration of object denoted by ID",
		Long:             "Displays the low-level information on an object identified by name or ID",
		TraverseChildren: true,
		RunE:             inspect,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
	})
	inspectOpts = common.AddInspectFlagSet(inspectCmd)
}

func inspect(cmd *cobra.Command, args []string) error {
	// First check if the input is even valid for an image
	if _, err := reference.Parse(args[0]); err == nil {
		if found, err := registry.ImageEngine().Exists(context.Background(), args[0]); err != nil {
			return err
		} else if found.Value {
			return images.Inspect(cmd, args, inspectOpts)
		}
	}
	if found, err := registry.ContainerEngine().ContainerExists(context.Background(), args[0]); err != nil {
		return err
	} else if found.Value {
		return containers.Inspect(cmd, args, inspectOpts)
	}
	return fmt.Errorf("%s not found on system", args[0])
}
