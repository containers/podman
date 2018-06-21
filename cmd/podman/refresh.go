package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/urfave/cli"
)

var (
	refreshFlags = []cli.Flag{}

	refreshDescription = "The refresh command resets the state of all containers to handle database changes after a Podman upgrade. All running containers will be restarted."

	refreshCommand = cli.Command{
		Name:                   "refresh",
		Usage:                  "Refresh container state",
		Description:            refreshDescription,
		Flags:                  refreshFlags,
		Action:                 refreshCmd,
		UseShortOptionHandling: true,
	}
)

func refreshCmd(c *cli.Context) error {
	if len(c.Args()) > 0 {
		return errors.Errorf("refresh does not accept any arguments")
	}

	if err := validateFlags(c, refreshFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
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
