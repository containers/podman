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
	unpauseFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "unpause all paused containers",
		},
	}
	unpauseDescription = `
   podman unpause

   Unpauses one or more running containers.  The container name or ID can be used.
`
	unpauseCommand = cli.Command{
		Name:         "unpause",
		Usage:        "Unpause the processes in one or more containers",
		Description:  unpauseDescription,
		Flags:        unpauseFlags,
		Action:       unpauseCmd,
		ArgsUsage:    "CONTAINER-NAME [CONTAINER-NAME ...]",
		OnUsageError: usageErrorHandler,
	}
)

func unpauseCmd(c *cli.Context) error {
	var (
		unpauseContainers []*libpod.Container
		unpauseFuncs      []shared.ParallelWorkerInput
	)
	if os.Geteuid() != 0 {
		return errors.New("unpause is not supported for rootless containers")
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
		cs, err := getAllOrLatestContainers(c, runtime, libpod.ContainerStatePaused, "paused")
		if err != nil {
			return err
		}
		unpauseContainers = append(unpauseContainers, cs...)
	} else {
		for _, arg := range args {
			ctr, err := runtime.LookupContainer(arg)
			if err != nil {
				return err
			}
			unpauseContainers = append(unpauseContainers, ctr)
		}
	}

	// Assemble the unpause funcs
	for _, ctr := range unpauseContainers {
		con := ctr
		f := func() error {
			return con.Unpause()
		}

		unpauseFuncs = append(unpauseFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}

	maxWorkers := shared.Parallelize("unpause")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalInt("max-workers")
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	unpauseErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, unpauseFuncs)
	return printParallelOutput(unpauseErrors, errCount)
}
