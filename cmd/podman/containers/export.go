package containers

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/parse"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
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
	outputFile string
)

func exportFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	outputFlagName := "output"
	flags.StringVarP(&outputFile, outputFlagName, "o", "", "Write to a specified file (default: stdout, which must be redirected)")
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
	if len(outputFile) == 0 {
		file := os.Stdout
		if term.IsTerminal(int(file.Fd())) {
			return errors.New("refusing to export to terminal. Use -o flag or redirect")
		}
		exportOpts.Output = file
	} else {
		if err := parse.ValidateFileName(outputFile); err != nil {
			return err
		}
		// open file here with O_WRONLY since on MacOS it can fail to open /dev/stderr in read mode for example
		// https://github.com/containers/podman/issues/16870
		file, err := os.OpenFile(outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		exportOpts.Output = file
	}
	return registry.ContainerEngine().ContainerExport(context.Background(), strings.TrimPrefix(args[0], "/"), exportOpts)
}
