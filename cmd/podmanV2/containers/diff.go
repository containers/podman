package containers

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/report"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	// podman container _diff_
	diffCmd = &cobra.Command{
		Use:     "diff [flags] CONTAINER",
		Args:    registry.IdOrLatestArgs,
		Short:   "Inspect changes on container's file systems",
		Long:    `Displays changes on a container filesystem.  The container will be compared to its parent layer.`,
		PreRunE: preRunE,
		RunE:    diff,
		Example: `podman container diff myCtr
  podman container diff -l --format json myCtr`,
	}
	diffOpts *entities.DiffOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: diffCmd,
		Parent:  containerCmd,
	})

	diffOpts = &entities.DiffOptions{}
	flags := diffCmd.Flags()
	flags.BoolVar(&diffOpts.Archive, "archive", true, "Save the diff as a tar archive")
	_ = flags.MarkHidden("archive")
	flags.StringVar(&diffOpts.Format, "format", "", "Change the output format")

	if !registry.IsRemote() {
		flags.BoolVarP(&diffOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	}
}

func diff(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && !diffOpts.Latest {
		return errors.New("container must be specified: podman container diff [options [...]] ID-NAME")
	}

	results, err := registry.ContainerEngine().ContainerDiff(registry.GetContext(), args[0], entities.DiffOptions{})
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
