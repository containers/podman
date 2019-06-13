package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	cpCommand cliconfig.CpValues

	cpDescription = `Command copies the contents of SRC_PATH to the DEST_PATH.

  You can copy from the container's file system to the local machine or the reverse, from the local filesystem to the container. If "-" is specified for either the SRC_PATH or DEST_PATH, you can also stream a tar archive from STDIN or to STDOUT. The CONTAINER can be a running or stopped container.  The SRC_PATH or DEST_PATH can be a file or directory.
`
	_cpCommand = &cobra.Command{
		Use:   "cp [flags] SRC_PATH DEST_PATH",
		Short: "Copy files/folders between a container and the local filesystem",
		Long:  cpDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			cpCommand.InputArgs = args
			cpCommand.GlobalFlags = MainGlobalOpts
			cpCommand.Remote = remoteclient
			return cpCmd(&cpCommand)
		},
		Example: "[CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
	}
)

func init() {
	cpCommand.Command = _cpCommand
	flags := cpCommand.Flags()
	flags.BoolVar(&cpCommand.Extract, "extract", false, "Extract the tar file into the destination directory.")
	flags.BoolVar(&cpCommand.Pause, "pause", false, "Pause the container while copying")
	cpCommand.SetHelpTemplate(HelpTemplate())
	cpCommand.SetUsageTemplate(UsageTemplate())
	rootCmd.AddCommand(cpCommand.Command)
}

func cpCmd(c *cliconfig.CpValues) error {
	args := c.InputArgs
	if len(args) != 2 {
		return errors.Errorf("you must provide a source path and a destination path")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	return runtime.CopyBetweenHostAndContainer(args[0], args[1], c.Extract, c.Pause)
}
