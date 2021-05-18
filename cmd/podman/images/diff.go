package images

import (
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// podman container _inspect_
	diffCmd = &cobra.Command{
		Use:               "diff [options] IMAGE",
		Args:              cobra.ExactArgs(1),
		Short:             "Inspect changes to the image's file systems",
		Long:              `Displays changes to the image's filesystem.  The image will be compared to its parent layer.`,
		RunE:              diff,
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
	diffOpts = &entities.DiffOptions{}
	flags.BoolVar(&diffOpts.Archive, "archive", true, "Save the diff as a tar archive")
	_ = flags.MarkDeprecated("archive", "Provided for backwards compatibility, has no impact on output.")

	formatFlagName := "format"
	flags.StringVar(&diffOpts.Format, formatFlagName, "", "Change the output format")
	_ = diffCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))
}

func diff(cmd *cobra.Command, args []string) error {
	if diffOpts.Latest {
		return errors.New("image diff does not support --latest")
	}

	results, err := registry.ImageEngine().Diff(registry.GetContext(), args[0], *diffOpts)
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(diffOpts.Format):
		return common.ChangesToJSON(results)
	case diffOpts.Format == "":
		return common.ChangesToTable(results)
	default:
		return errors.New("only supported value for '--format' is 'json'")
	}
}

func Diff(cmd *cobra.Command, args []string, options entities.DiffOptions) error {
	diffOpts = &options
	return diff(cmd, args)
}
