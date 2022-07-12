package containers

import (
	"context"
	"errors"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/parse"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	exportDescription = "Exports container's filesystem contents as a tar archive" +
		" and saves it on the local machine."

	exportCommand = &cobra.Command{
		Use:               "export [options] CONTAINER",
		Short:             "Export container's filesystem contents as a tar archive",
		Long:              exportDescription,
		RunE:              export,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman export ctrID > myCtr.tar
  podman export --output="myCtr.tar" ctrID`,
	}

	containerExportCommand = &cobra.Command{
		Args:              cobra.ExactArgs(1),
		Use:               exportCommand.Use,
		Short:             exportCommand.Short,
		Long:              exportCommand.Long,
		RunE:              exportCommand.RunE,
		ValidArgsFunction: exportCommand.ValidArgsFunction,
		Example: `podman container export ctrID > myCtr.tar
  podman container export --output="myCtr.tar" ctrID`,
	}
)

var (
	exportOpts entities.ContainerExportOptions
)

func exportFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	outputFlagName := "output"
	flags.StringVarP(&exportOpts.Output, outputFlagName, "o", "", "Write to a specified file (default: stdout, which must be redirected)")
	_ = cmd.RegisterFlagCompletionFunc(outputFlagName, completion.AutocompleteDefault)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: exportCommand,
	})
	exportFlags(exportCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerExportCommand,
		Parent:  containerCmd,
	})
	exportFlags(containerExportCommand)
}

func export(cmd *cobra.Command, args []string) error {
	if len(exportOpts.Output) == 0 {
		file := os.Stdout
		if term.IsTerminal(int(file.Fd())) {
			return errors.New("refusing to export to terminal. Use -o flag or redirect")
		}
		exportOpts.Output = "/dev/stdout"
	} else if err := parse.ValidateFileName(exportOpts.Output); err != nil {
		return err
	}
	return registry.ContainerEngine().ContainerExport(context.Background(), args[0], exportOpts)
}
