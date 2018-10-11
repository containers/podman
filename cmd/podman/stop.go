package main

import (
	"fmt"
	"os"
	rt "runtime"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	stopFlags = []cli.Flag{
		cli.UintFlag{
			Name:  "timeout, time, t",
			Usage: "Seconds to wait for stop before killing the container",
			Value: libpod.CtrRemoveTimeout,
		},
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "stop all running containers",
		}, LatestFlag,
	}
	stopDescription = `
   podman stop

   Stops one or more running containers.  The container name or ID can be used.
   A timeout to forcibly stop the container can also be set but defaults to 10
   seconds otherwise.
`

	stopCommand = cli.Command{
		Name:         "stop",
		Usage:        "Stop one or more containers",
		Description:  stopDescription,
		Flags:        sortFlags(stopFlags),
		Action:       stopCmd,
		ArgsUsage:    "CONTAINER-NAME [CONTAINER-NAME ...]",
		OnUsageError: usageErrorHandler,
	}
)

func stopCmd(c *cli.Context) error {
	args := c.Args()
	if (c.Bool("all") || c.Bool("latest")) && len(args) > 0 {
		return errors.Errorf("no arguments are needed with --all or --latest")
	}
	if c.Bool("all") && c.Bool("latest") {
		return errors.Errorf("--all and --latest cannot be used together")
	}
	if len(args) < 1 && !c.Bool("all") && !c.Bool("latest") {
		return errors.Errorf("you must provide at least one container name or id")
	}
	if err := validateFlags(c, stopFlags); err != nil {
		return err
	}

	rootless.SetSkipStorageSetup(true)
	runtime, err := libpodruntime.GetRuntime(c)
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
	} else if c.Bool("latest") {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return errors.Wrapf(err, "unable to get last created container")
		}
		containers = append(containers, lastCtr)
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

	var stopFuncs []workerInput
	for _, ctr := range containers {
		con := ctr
		var stopTimeout uint
		if c.IsSet("timeout") {
			stopTimeout = c.Uint("timeout")
		} else {
			stopTimeout = ctr.StopTimeout()
		}
		f := func() error {
			return con.StopWithTimeout(stopTimeout)
		}
		stopFuncs = append(stopFuncs, workerInput{
			containerID:  con.ID(),
			parallelFunc: f,
		})
	}

	stopErrors := parallelExecuteWorkerPool(rt.NumCPU()*3, stopFuncs)

	for cid, result := range stopErrors {
		if result != nil && result != libpod.ErrCtrStopped {
			fmt.Println(result.Error())
			lastError = result
			continue
		}
		fmt.Println(cid)
	}
	return lastError
}
