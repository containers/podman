package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	restartCommand     cliconfig.RestartValues
	restartDescription = `Restarts one or more running containers. The container ID or name can be used. A timeout before forcibly stopping can be set, but defaults to 10 seconds`
	_restartCommand    = &cobra.Command{
		Use:   "restart",
		Short: "Restart one or more containers",
		Long:  restartDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			restartCommand.InputArgs = args
			restartCommand.GlobalFlags = MainGlobalOpts
			return restartCmd(&restartCommand)
		},
		Example: "CONTAINER [CONTAINER ...]",
	}
)

func init() {
	restartCommand.Command = _restartCommand
	flags := restartCommand.Flags()
	flags.BoolVarP(&restartCommand.All, "all", "a", false, "Restart all non-running containers")
	flags.BoolVarP(&restartCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&restartCommand.Running, "running", false, "Restart only running containers when --all is used")
	flags.UintVarP(&restartCommand.Timeout, "timeout", "t", libpod.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
	flags.UintVar(&restartCommand.Timeout, "time", libpod.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")

	rootCmd.AddCommand(restartCommand.Command)
}

func restartCmd(c *cliconfig.RestartValues) error {
	var (
		restartFuncs      []shared.ParallelWorkerInput
		containers        []*libpod.Container
		restartContainers []*libpod.Container
	)

	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	args := c.InputArgs
	runOnly := c.Running
	all := c.All
	if len(args) < 1 && !c.Latest && !all {
		return errors.Wrapf(libpod.ErrInvalidArg, "you must provide at least one container name or ID")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	timeout := c.Timeout
	useTimeout := c.Flag("timeout").Changed

	// Handle --latest
	if c.Latest {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest container")
		}
		restartContainers = append(restartContainers, lastCtr)
	} else if runOnly {
		containers, err = getAllOrLatestContainers(&c.PodmanCommand, runtime, libpod.ContainerStateRunning, "running")
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
		maxWorkers = c.GlobalFlags.MaxWorks
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
