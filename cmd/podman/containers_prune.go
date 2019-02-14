package main

import (
	"context"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	pruneContainersCommand     cliconfig.PruneContainersValues
	pruneContainersDescription = `
	podman container prune

	Removes all exited containers
`
	_pruneContainersCommand = &cobra.Command{
		Use:   "prune",
		Short: "Remove all stopped containers",
		Long:  pruneContainersDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pruneContainersCommand.InputArgs = args
			pruneContainersCommand.GlobalFlags = MainGlobalOpts
			return pruneContainersCmd(&pruneContainersCommand)
		},
	}
)

func init() {
	pruneContainersCommand.Command = _pruneContainersCommand
	pruneContainersCommand.SetUsageTemplate(UsageTemplate())
	flags := pruneContainersCommand.Flags()
	flags.BoolVarP(&pruneContainersCommand.Force, "force", "f", false, "Force removal of a running container.  The default is false")
}

func pruneContainers(runtime *adapter.LocalRuntime, ctx context.Context, maxWorkers int, force, volumes bool) error {
	var deleteFuncs []shared.ParallelWorkerInput

	filter := func(c *libpod.Container) bool {
		state, err := c.State()
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
			return runtime.RemoveContainer(ctx, con, force, volumes)
		}

		deleteFuncs = append(deleteFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}
	// Run the parallel funcs
	deleteErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, deleteFuncs)
	return printParallelOutput(deleteErrors, errCount)
}

func pruneContainersCmd(c *cliconfig.PruneContainersValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	maxWorkers := shared.Parallelize("rm")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	return pruneContainers(runtime, getContext(), maxWorkers, c.Bool("force"), c.Bool("volumes"))
}
