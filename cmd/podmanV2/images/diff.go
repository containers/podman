package images

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/report"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	// podman container _inspect_
	diffCmd = &cobra.Command{
		Use:     "diff [flags] CONTAINER",
		Args:    registry.IdOrLatestArgs,
		Short:   "Inspect changes on image's file systems",
		Long:    `Displays changes on a image's filesystem.  The image will be compared to its parent layer.`,
		PreRunE: preRunE,
		RunE:    diff,
		Example: `podman image diff myImage
  podman image diff --format json redis:alpine`,
	}
	diffOpts *entities.DiffOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: diffCmd,
		Parent:  imageCmd,
	})

	diffOpts = &entities.DiffOptions{}
	flags := diffCmd.Flags()
	flags.BoolVar(&diffOpts.Archive, "archive", true, "Save the diff as a tar archive")
	_ = flags.MarkHidden("archive")
	flags.StringVar(&diffOpts.Format, "format", "", "Change the output format")
}

func diff(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && !diffOpts.Latest {
		return errors.New("image must be specified: podman image diff [options [...]] ID-NAME")
	}

	results, err := registry.ImageEngine().Diff(registry.GetContext(), args[0], entities.DiffOptions{})
	if err != nil {
		return err
	}

	switch diffOpts.Format {
	case "":
		return report.ChangesToTable(results)
	case "json":
		return report.ChangesToJSON(results)
	default:
		return errors.New("only supported value for '--format' is 'json'")
	}
}

func Diff(cmd *cobra.Command, args []string, options entities.DiffOptions) error {
	diffOpts = &options
	return diff(cmd, args)
}
