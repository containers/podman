package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

var (
	stopFlags = []cli.Flag{
		cli.UintFlag{
			Name:  "timeout, t",
			Usage: "Seconds to wait for stop before killing the container",
			Value: libpod.CtrRemoveTimeout,
		},
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "stop all running containers",
		},
	}
	stopDescription = `
   podman stop

   Stops one or more running containers.  The container name or ID can be used.
   A timeout to forcibly stop the container can also be set but defaults to 10
   seconds otherwise.
`

	stopCommand = cli.Command{
		Name:        "stop",
		Usage:       "Stop one or more containers",
		Description: stopDescription,
		Flags:       stopFlags,
		Action:      stopCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func stopCmd(c *cli.Context) error {
	args := c.Args()
	if c.Bool("all") && len(args) > 0 {
		return errors.Errorf("no arguments are needed with -a")
	}
	if len(args) < 1 && !c.Bool("all") {
		return errors.Errorf("you must provide at least one container name or id")
	}
	if err := validateFlags(c, stopFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var filterFuncs []libpod.ContainerFilter
	var containers []*libpod.Container
	var lastError error

	if c.Bool("all") {
		// only get running containers
		filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == libpod.ContainerStateRunning
		})
		containers, err = runtime.GetContainers(filterFuncs...)
		if err != nil {
			return errors.Wrapf(err, "unable to get running containers")
		}
	} else {
		for _, i := range args {
			container, err := runtime.LookupContainer(i)
			if err != nil {
				if lastError != nil {
					fmt.Fprintln(os.Stderr, lastError)
				}
				lastError = errors.Wrapf(err, "unable to find container %s", i)
				continue
			}
			containers = append(containers, container)
		}
	}

	for _, ctr := range containers {
		var stopTimeout uint
		if c.IsSet("timeout") {
			stopTimeout = c.Uint("timeout")
		} else {
			stopTimeout = ctr.StopTimeout()
		}
		if err := ctr.Stop(stopTimeout); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to stop container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
