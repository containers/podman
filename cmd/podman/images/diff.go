package images

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/diff"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// podman container _inspect_
	diffCmd = &cobra.Command{
		Use:               "diff [options] IMAGE [IMAGE]",
		Args:              cobra.RangeArgs(1, 2),
		Short:             "Inspect changes to the image's file systems",
		Long:              `Displays changes to the image's filesystem.  The image will be compared to its parent layer or the second argument when given.`,
		RunE:              diffRun,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image diff myImage
  podman image diff --format json redis:alpine`,
	}
	diffOpts *entities.DiffOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: diffCmd,
		Parent:  imageCmd,
	})
	diffFlags(diffCmd.Flags())
}

func diffFlags(flags *pflag.FlagSet) {
	diffOpts = new(entities.DiffOptions)

	formatFlagName := "format"
	flags.StringVar(&diffOpts.Format, formatFlagName, "", "Change the output format (json)")
	_ = diffCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))
}

func diffRun(cmd *cobra.Command, args []string) error {
	diffOpts.Type = define.DiffImage
	return diff.Diff(cmd, args, *diffOpts)
}
