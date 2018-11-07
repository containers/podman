package main

import (
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	pauseFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "pause all running containers",
		},
	}
	pauseDescription = `
   podman pause

   Pauses one or more running containers.  The container name or ID can be used.
`
	pauseCommand = cli.Command{
		Name:         "pause",
		Usage:        "Pauses all the processes in one or more containers",
		Description:  pauseDescription,
		Flags:        pauseFlags,
		Action:       pauseCmd,
		ArgsUsage:    "CONTAINER-NAME [CONTAINER-NAME ...]",
		OnUsageError: usageErrorHandler,
	}
)

func pauseCmd(c *cli.Context) error {
	var (
		pauseContainers []*libpod.Container
		pauseFuncs      []shared.ParallelWorkerInput
	)
	if os.Geteuid() != 0 {
		return errors.New("pause is not supported for rootless containers")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) < 1 && !c.Bool("all") {
		return errors.Errorf("you must provide at least one container name or id")
	}
	if c.Bool("all") {
		containers, err := getAllOrLatestContainers(c, runtime, libpod.ContainerStateRunning, "running")
		if err != nil {
			return err
		}
		pauseContainers = append(pauseContainers, containers...)
	} else {
		for _, arg := range args {
			ctr, err := runtime.LookupContainer(arg)
			if err != nil {
				return err
			}
			pauseContainers = append(pauseContainers, ctr)
		}
	}

	// Now assemble the slice of pauseFuncs
	for _, ctr := range pauseContainers {
		con := ctr

		f := func() error {
			return con.Pause()
		}
		pauseFuncs = append(pauseFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}

	maxWorkers := shared.Parallelize("pause")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalInt("max-workers")
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	pauseErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, pauseFuncs)
	return printParallelOutput(pauseErrors, errCount)
}
