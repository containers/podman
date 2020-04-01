package containers

import (
	"context"
	"os"

	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	exportDescription = "Exports container's filesystem contents as a tar archive" +
		" and saves it on the local machine."

	exportCommand = &cobra.Command{
		Use:               "export [flags] CONTAINER",
		Short:             "Export container's filesystem contents as a tar archive",
		Long:              exportDescription,
		PersistentPreRunE: preRunE,
		RunE:              export,
		Args:              cobra.ExactArgs(1),
		Example: `podman export ctrID > myCtr.tar
  podman export --output="myCtr.tar" ctrID`,
	}
)

var (
	exportOpts entities.ContainerExportOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: exportCommand,
	})
	exportCommand.SetHelpTemplate(registry.HelpTemplate())
	exportCommand.SetUsageTemplate(registry.UsageTemplate())
	flags := exportCommand.Flags()
	flags.StringVarP(&exportOpts.Output, "output", "o", "", "Write to a specified file (default: stdout, which must be redirected)")
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
