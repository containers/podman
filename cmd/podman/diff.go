package main

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/diff"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// Inspect is one of the outlier commands in that it operates on images/containers/...

var (
	// Command: podman _diff_ Object_ID
	diffDescription = `Displays changes on a container or image's filesystem.  The container or image will be compared to its parent layer or the second argument when given.`
	diffCmd         = &cobra.Command{
		Use:               "diff [options] {CONTAINER|IMAGE} [{CONTAINER|IMAGE}]",
		Args:              diff.ValidateContainerDiffArgs,
		Short:             "Display the changes to the object's file system",
		Long:              diffDescription,
		RunE:              diffRun,
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

	formatFlagName := "format"
	flags.StringVar(&diffOpts.Format, formatFlagName, "", "Change the output format (json)")
	_ = diffCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))

	validate.AddLatestFlag(diffCmd, &diffOpts.Latest)
}

func diffRun(cmd *cobra.Command, args []string) error {
	diffOpts.Type = define.DiffAll
	return diff.Diff(cmd, args, diffOpts)
}
