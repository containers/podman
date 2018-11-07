package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

	if err := checkAllAndLatest(c); err != nil {
		return err
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

	containers, err := getAllOrLatestContainers(c, runtime, libpod.ContainerStateRunning, "running")
	if err != nil {
		if len(containers) == 0 {
			return err
		}
		fmt.Println(err.Error())
	}

	var stopFuncs []shared.ParallelWorkerInput
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
		stopFuncs = append(stopFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}

	maxWorkers := shared.Parallelize("stop")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalInt("max-workers")
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	stopErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, stopFuncs)
	return printParallelOutput(stopErrors, errCount)
}
