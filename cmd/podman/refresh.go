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
	refreshDescription = "The refresh command resets the state of all containers to handle database changes after a Podman upgrade. All running containers will be restarted."
	_refreshCommand    = &cobra.Command{
		Use:   "refresh",
		Short: "Refresh container state",
		Long:  refreshDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			refreshCommand.InputArgs = args
			refreshCommand.GlobalFlags = MainGlobalOpts
			return refreshCmd(&refreshCommand)
		},
	}
)

func init() {
	refreshCommand.Command = _refreshCommand
	rootCmd.AddCommand(refreshCommand.Command)
}

func refreshCmd(c *cliconfig.RefreshValues) error {
	if len(c.InputArgs) > 0 {
		return errors.Errorf("refresh does not accept any arguments")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
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
