package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	restartFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "restart all non-running containers",
		},
		cli.BoolFlag{
			Name:  "running",
			Usage: "restart only running containers when --all is used",
		},
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
		Flags:                  sortFlags(restartFlags),
		Action:                 restartCmd,
		ArgsUsage:              "CONTAINER [CONTAINER ...]",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

func restartCmd(c *cli.Context) error {
	var (
		restartFuncs      []shared.ParallelWorkerInput
		containers        []*libpod.Container
		restartContainers []*libpod.Container
	)

	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	args := c.Args()
	runOnly := c.Bool("running")
	all := c.Bool("all")
	if len(args) < 1 && !c.Bool("latest") && !all {
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

	timeout := c.Uint("timeout")
	useTimeout := c.IsSet("timeout")

	// Handle --latest
	if c.Bool("latest") {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest container")
		}
		restartContainers = append(restartContainers, lastCtr)
	} else if runOnly {
		containers, err = getAllOrLatestContainers(c, runtime, libpod.ContainerStateRunning, "running")
		if err != nil {
			return err
		}
		restartContainers = append(restartContainers, containers...)
	} else if all {
		containers, err = runtime.GetAllContainers()
		if err != nil {
			return err
		}
		restartContainers = append(restartContainers, containers...)
	} else {
		for _, id := range args {
			ctr, err := runtime.LookupContainer(id)
			if err != nil {
				return err
			}
			restartContainers = append(restartContainers, ctr)
		}
	}

	maxWorkers := shared.Parallelize("restart")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalInt("max-workers")
	}

	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	if rootless.IsRootless() {
		// With rootless containers we cannot really restart an existing container
		// as we would need to join the mount namespace as well to be able to reuse
		// the storage.
		if err := stopRootlessContainers(restartContainers, timeout, useTimeout, maxWorkers); err != nil {
			return err
		}
		became, ret, err := rootless.BecomeRootInUserNS()
		if err != nil {
			return err
		}
		if became {
			os.Exit(ret)
		}
	}

	// We now have a slice of all the containers to be restarted. Iterate them to
	// create restart Funcs with a timeout as needed
	for _, ctr := range restartContainers {
		con := ctr
		ctrTimeout := ctr.StopTimeout()
		if useTimeout {
			ctrTimeout = timeout
		}

		f := func() error {
			return con.RestartWithTimeout(getContext(), ctrTimeout)
		}

		restartFuncs = append(restartFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}

	restartErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, restartFuncs)
	return printParallelOutput(restartErrors, errCount)
}

func stopRootlessContainers(stopContainers []*libpod.Container, timeout uint, useTimeout bool, maxWorkers int) error {
	var stopFuncs []shared.ParallelWorkerInput
	for _, ctr := range stopContainers {
		state, err := ctr.State()
		if err != nil {
			return err
		}
		if state != libpod.ContainerStateRunning {
			continue
		}

		ctrTimeout := ctr.StopTimeout()
		if useTimeout {
			ctrTimeout = timeout
		}

		c := ctr
		f := func() error {
			return c.StopWithTimeout(ctrTimeout)
		}

		stopFuncs = append(stopFuncs, shared.ParallelWorkerInput{
			ContainerID:  c.ID(),
			ParallelFunc: f,
		})

		restartErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, stopFuncs)
		var lastError error
		for _, result := range restartErrors {
			if result != nil {
				if errCount > 1 {
					fmt.Println(result.Error())
				}
				lastError = result
			}
		}
		if lastError != nil {
			return lastError
		}
	}
	return nil
}
