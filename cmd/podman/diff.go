package main

import (
	"fmt"

	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/containers"
	"github.com/containers/podman/v3/cmd/podman/images"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// Inspect is one of the outlier commands in that it operates on images/containers/...

var (
	// Command: podman _diff_ Object_ID
	diffDescription = `Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer.`
	diffCmd         = &cobra.Command{
		Use:               "diff [options] {CONTAINER|IMAGE}",
		Args:              validate.IDOrLatestArgs,
		Short:             "Display the changes to the object's file system",
		Long:              diffDescription,
		RunE:              diff,
		ValidArgsFunction: common.AutocompleteContainersAndImages,
		Example: `podman diff imageID
  podman diff ctrID
  podman diff --format json redis:alpine`,
	}

	diffOpts = entities.DiffOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: diffCmd,
	})
	flags := diffCmd.Flags()
	flags.BoolVar(&diffOpts.Archive, "archive", true, "Save the diff as a tar archive")
	_ = flags.MarkHidden("archive")

	formatFlagName := "format"
	flags.StringVar(&diffOpts.Format, formatFlagName, "", "Change the output format")
	_ = diffCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))

	validate.AddLatestFlag(diffCmd, &diffOpts.Latest)
}

func diff(cmd *cobra.Command, args []string) error {
	// Latest implies looking for a container
	if diffOpts.Latest {
		return containers.Diff(cmd, args, diffOpts)
	}

	options := entities.ContainerExistsOptions{
		External: true,
	}
	if found, err := registry.ContainerEngine().ContainerExists(registry.GetContext(), args[0], options); err != nil {
		return err
	} else if found.Value {
		return containers.Diff(cmd, args, diffOpts)
	}

	if found, err := registry.ImageEngine().Exists(registry.GetContext(), args[0]); err != nil {
		return err
	} else if found.Value {
		return images.Diff(cmd, args, diffOpts)
	}

	return fmt.Errorf("%s not found on system", args[0])
}
