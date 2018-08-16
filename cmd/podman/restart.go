package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	restartFlags = []cli.Flag{
		cli.UintFlag{
			Name:  "timeout, time, t",
			Usage: "Seconds to wait for stop before killing the container",
			Value: libpod.CtrRemoveTimeout,
		},
		LatestFlag,
	}
	restartDescription = `Restarts one or more running containers. The container ID or name can be used. A timeout before forcibly stopping can be set, but defaults to 10 seconds`

	restartCommand = cli.Command{
		Name:                   "restart",
		Usage:                  "Restart one or more containers",
		Description:            restartDescription,
		Flags:                  restartFlags,
		Action:                 restartCmd,
		ArgsUsage:              "CONTAINER [CONTAINER ...]",
		UseShortOptionHandling: true,
	}
)

func restartCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 && !c.Bool("latest") {
		return errors.Wrapf(libpod.ErrInvalidArg, "you must provide at least one container name or ID")
	}

	if err := validateFlags(c, restartFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	var lastError error

	timeout := c.Uint("timeout")
	useTimeout := c.IsSet("timeout")

	// Handle --latest
	if c.Bool("latest") {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			lastError = errors.Wrapf(err, "unable to get latest container")
		} else {
			ctrTimeout := lastCtr.StopTimeout()
			if useTimeout {
				ctrTimeout = timeout
			}

			lastError = lastCtr.RestartWithTimeout(context.TODO(), ctrTimeout)
		}
	}

	for _, id := range args {
		ctr, err := runtime.LookupContainer(id)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to find container %s", id)
			continue
		}

		ctrTimeout := ctr.StopTimeout()
		if useTimeout {
			ctrTimeout = timeout
		}

		if err := ctr.RestartWithTimeout(context.TODO(), ctrTimeout); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "error restarting container %s", ctr.ID())
		}
	}

	return lastError
}
