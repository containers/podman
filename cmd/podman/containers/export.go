package containers

import (
	"context"
	"os"

	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	exportDescription = "Exports container's filesystem contents as a tar archive" +
		" and saves it on the local machine."

	exportCommand = &cobra.Command{
		Use:   "export [options] CONTAINER",
		Short: "Export container's filesystem contents as a tar archive",
		Long:  exportDescription,
		RunE:  export,
		Args:  cobra.ExactArgs(1),
		Example: `podman export ctrID > myCtr.tar
  podman export --output="myCtr.tar" ctrID`,
	}

	containerExportCommand = &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   exportCommand.Use,
		Short: exportCommand.Short,
		Long:  exportCommand.Long,
		RunE:  exportCommand.RunE,
		Example: `podman container export ctrID > myCtr.tar
  podman container export --output="myCtr.tar" ctrID`,
	}
)

var (
	exportOpts entities.ContainerExportOptions
)

func exportFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&exportOpts.Output, "output", "o", "", "Write to a specified file (default: stdout, which must be redirected)")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: exportCommand,
	})
	flags := exportCommand.Flags()
	exportFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerExportCommand,
		Parent:  containerCmd,
	})

	containerExportFlags := containerExportCommand.Flags()
	exportFlags(containerExportFlags)
}

func export(cmd *cobra.Command, args []string) error {
	if len(exportOpts.Output) == 0 {
		file := os.Stdout
		if terminal.IsTerminal(int(file.Fd())) {
			return errors.Errorf("refusing to export to terminal. Use -o flag or redirect")
		}
		exportOpts.Output = "/dev/stdout"
	} else if err := parse.ValidateFileName(exportOpts.Output); err != nil {
		return err
	}
	return registry.ContainerEngine().ContainerExport(context.Background(), args[0], exportOpts)
}
