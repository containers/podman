package main

import (
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	pruneContainersDescription = `
	podman container prune

	Removes all exited containers
`

	pruneContainersCommand = cli.Command{
		Name:         "prune",
		Usage:        "Remove all stopped containers",
		Description:  pruneContainersDescription,
		Action:       pruneContainersCmd,
		OnUsageError: usageErrorHandler,
	}
)

func pruneContainersCmd(c *cli.Context) error {
	var (
		deleteFuncs []shared.ParallelWorkerInput
	)

	ctx := getContext()
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	filter := func(c *libpod.Container) bool {
		state, _ := c.State()
		if state == libpod.ContainerStateStopped || (state == libpod.ContainerStateExited && err == nil && c.PodID() == "") {
			return true
		}
		return false
	}
	delContainers, err := runtime.GetContainers(filter)
	if err != nil {
		return err
	}
	if len(delContainers) < 1 {
		return nil
	}
	for _, container := range delContainers {
		con := container
		f := func() error {
			return runtime.RemoveContainer(ctx, con, c.Bool("force"))
		}

		deleteFuncs = append(deleteFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}
	maxWorkers := shared.Parallelize("rm")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalInt("max-workers")
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	// Run the parallel funcs
	deleteErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, deleteFuncs)
	return printParallelOutput(deleteErrors, errCount)
}
