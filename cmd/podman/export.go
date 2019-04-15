package main

import (
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared/parse"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	exportCommand     cliconfig.ExportValues
	exportDescription = "Exports container's filesystem contents as a tar archive" +
		" and saves it on the local machine."

	_exportCommand = &cobra.Command{
		Use:   "export [flags] CONTAINER",
		Short: "Export container's filesystem contents as a tar archive",
		Long:  exportDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			exportCommand.InputArgs = args
			exportCommand.GlobalFlags = MainGlobalOpts
			exportCommand.Remote = remoteclient
			return exportCmd(&exportCommand)
		},
		Example: `podman export ctrID > myCtr.tar
  podman export --output="myCtr.tar" ctrID`,
	}
)

func init() {
	exportCommand.Command = _exportCommand
	exportCommand.SetHelpTemplate(HelpTemplate())
	exportCommand.SetUsageTemplate(UsageTemplate())
	flags := exportCommand.Flags()
	flags.StringVarP(&exportCommand.Output, "output", "o", "", "Write to a specified file (default: stdout, which must be redirected)")
}

// exportCmd saves a container to a tarball on disk
func exportCmd(c *cliconfig.ExportValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.InputArgs
	if len(args) == 0 {
		return errors.Errorf("container id must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments given, need 1 at most.")
	}

	output := c.Output
	if runtime.Remote && len(output) == 0 {
		return errors.New("remote client usage must specify an output file (-o)")
	}

	if len(output) == 0 {
		file := os.Stdout
		if logrus.IsTerminal(file) {
			return errors.Errorf("refusing to export to terminal. Use -o flag or redirect")
		}
		output = "/dev/stdout"
	}

	if err := parse.ValidateFileName(output); err != nil {
		return err
	}
	return runtime.Export(args[0], output)
}
