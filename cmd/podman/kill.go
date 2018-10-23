package main

import (
	"os"
	"syscall"

	"fmt"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	killFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Signal all running containers",
		},
		cli.StringFlag{
			Name:  "signal, s",
			Usage: "Signal to send to the container",
			Value: "KILL",
		},
		LatestFlag,
	}
	killDescription = "The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal."
	killCommand     = cli.Command{
		Name:                   "kill",
		Usage:                  "Kill one or more running containers with a specific signal",
		Description:            killDescription,
		Flags:                  sortFlags(killFlags),
		Action:                 killCmd,
		ArgsUsage:              "CONTAINER-NAME [CONTAINER-NAME ...]",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

// killCmd kills one or more containers with a signal
func killCmd(c *cli.Context) error {
	if err := checkAllAndLatest(c); err != nil {
		return err
	}

	if err := validateFlags(c, killFlags); err != nil {
		return err
	}

	rootless.SetSkipStorageSetup(true)
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var killSignal uint = uint(syscall.SIGTERM)
	if c.String("signal") != "" {
		// Check if the signalString provided by the user is valid
		// Invalid signals will return err
		sysSignal, err := signal.ParseSignal(c.String("signal"))
		if err != nil {
			return err
		}
		killSignal = uint(sysSignal)
	}

	containers, lastError := getAllOrLatestContainers(c, runtime, libpod.ContainerStateRunning, "running")

	for _, ctr := range containers {
		if err := ctr.Kill(killSignal); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to find container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
