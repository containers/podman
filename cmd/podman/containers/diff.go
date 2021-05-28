package containers

import (
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	// podman container _diff_
	diffCmd = &cobra.Command{
		Use:               "diff [options] CONTAINER",
		Args:              validate.IDOrLatestArgs,
		Short:             "Inspect changes to the container's file systems",
		Long:              `Displays changes to the container filesystem's'.  The container will be compared to its parent layer.`,
		RunE:              diff,
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman container diff myCtr
  podman container diff -l --format json myCtr`,
	}
	diffOpts *entities.DiffOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: diffCmd,
		Parent:  containerCmd,
	})

	diffOpts = &entities.DiffOptions{}
	flags := diffCmd.Flags()
	flags.BoolVar(&diffOpts.Archive, "archive", true, "Save the diff as a tar archive")
	_ = flags.MarkHidden("archive")

	formatFlagName := "format"
	flags.StringVar(&diffOpts.Format, formatFlagName, "", "Change the output format")
	_ = diffCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))

	validate.AddLatestFlag(diffCmd, &diffOpts.Latest)
}

func diff(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && !diffOpts.Latest {
		return errors.New("container must be specified: podman container diff [options [...]] ID-NAME")
	}

	var id string
	if len(args) > 0 {
		id = args[0]
	}
	results, err := registry.ContainerEngine().ContainerDiff(registry.GetContext(), id, *diffOpts)
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
