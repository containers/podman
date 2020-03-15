package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	systemResetCommand     cliconfig.SystemResetValues
	systemResetDescription = `Reset podman storage back to default state"

  All containers will be stopped and removed, and all images, volumes and container content will be removed.
`
	_systemResetCommand = &cobra.Command{
		Use:   "reset",
		Args:  noSubArgs,
		Short: "Reset podman storage",
		Long:  systemResetDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			systemResetCommand.InputArgs = args
			systemResetCommand.GlobalFlags = MainGlobalOpts
			systemResetCommand.Remote = remoteclient
			return systemResetCmd(&systemResetCommand)
		},
	}
)

func init() {
	systemResetCommand.Command = _systemResetCommand
	flags := systemResetCommand.Flags()
	flags.BoolVarP(&systemResetCommand.Force, "force", "f", false, "Do not prompt for confirmation")

	systemResetCommand.SetHelpTemplate(HelpTemplate())
	systemResetCommand.SetUsageTemplate(UsageTemplate())
}

func systemResetCmd(c *cliconfig.SystemResetValues) error {
	// Prompt for confirmation if --force is not set
	if !c.Force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(`
WARNING! This will remove:
        - all containers
        - all pods
        - all images
        - all build cache
Are you sure you want to continue? [y/N] `)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	// No shutdown, since storage will be destroyed when command completes

	return runtime.Reset()
}
