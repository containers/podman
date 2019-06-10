package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	refreshCommand     cliconfig.RefreshValues
	refreshDescription = `Resets the state of all containers to handle database changes after a Podman upgrade.

  All running containers will be restarted.
`
	_refreshCommand = &cobra.Command{
		Use:   "refresh",
		Args:  noSubArgs,
		Short: "Refresh container state",
		Long:  refreshDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			refreshCommand.InputArgs = args
			refreshCommand.GlobalFlags = MainGlobalOpts
			refreshCommand.Remote = remoteclient
			return refreshCmd(&refreshCommand)
		},
	}
)

func init() {
	_refreshCommand.Hidden = true
	refreshCommand.Command = _refreshCommand
	refreshCommand.SetHelpTemplate(HelpTemplate())
	refreshCommand.SetUsageTemplate(UsageTemplate())
}

func refreshCmd(c *cliconfig.RefreshValues) error {
	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	allCtrs, err := runtime.GetAllContainers()
	if err != nil {
		return err
	}

	ctx := getContext()

	var lastError error
	for _, ctr := range allCtrs {
		if err := ctr.Refresh(ctx); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "error refreshing container %s state", ctr.ID())
		}
	}

	return lastError
}
