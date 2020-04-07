package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/containers"
	"github.com/containers/libpod/cmd/podmanV2/images"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// Inspect is one of the outlier commands in that it operates on images/containers/...

var (
	// Command: podman _diff_ Object_ID
	diffDescription = `Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer.`
	diffCmd         = &cobra.Command{
		Use:              "diff [flags] {CONTAINER_ID | IMAGE_ID}",
		Args:             registry.IdOrLatestArgs,
		Short:            "Display the changes of object's file system",
		Long:             diffDescription,
		TraverseChildren: true,
		RunE:             diff,
		Example: `podman diff imageID
  podman diff ctrID
  podman diff --format json redis:alpine`,
	}

	diffOpts = entities.DiffOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: diffCmd,
	})
	diffCmd.SetHelpTemplate(registry.HelpTemplate())
	diffCmd.SetUsageTemplate(registry.UsageTemplate())

	flags := diffCmd.Flags()
	flags.BoolVar(&diffOpts.Archive, "archive", true, "Save the diff as a tar archive")
	_ = flags.MarkHidden("archive")
	flags.StringVar(&diffOpts.Format, "format", "", "Change the output format")

	if !registry.IsRemote() {
		flags.BoolVarP(&diffOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	}
}

func diff(cmd *cobra.Command, args []string) error {
	ie, err := registry.NewImageEngine(cmd, args)
	if err != nil {
		return err
	}

	if found, err := ie.Exists(registry.GetContext(), args[0]); err != nil {
		return err
	} else if found.Value {
		return images.Diff(cmd, args, diffOpts)
	}

	ce, err := registry.NewContainerEngine(cmd, args)
	if err != nil {
		return err
	}

	if found, err := ce.ContainerExists(registry.GetContext(), args[0]); err != nil {
		return err
	} else if found.Value {
		return containers.Diff(cmd, args, diffOpts)
	}
	return fmt.Errorf("%s not found on system", args[0])
}
