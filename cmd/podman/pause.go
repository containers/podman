package main

import (
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	pauseCommand     cliconfig.PauseValues
	pauseDescription = `
   podman pause

   Pauses one or more running containers.  The container name or ID can be used.
`
	_pauseCommand = &cobra.Command{
		Use:   "pause",
		Short: "Pause all the processes in one or more containers",
		Long:  pauseDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pauseCommand.InputArgs = args
			pauseCommand.GlobalFlags = MainGlobalOpts
			return pauseCmd(&pauseCommand)
		},
		Example: `podman pause mywebserver
  podman pause 860a4b23
  podman stop -a`,
	}
)

func init() {
	pauseCommand.Command = _pauseCommand
	pauseCommand.SetUsageTemplate(UsageTemplate())
	flags := pauseCommand.Flags()
	flags.BoolVarP(&pauseCommand.All, "all", "a", false, "Pause all running containers")

}

func pauseCmd(c *cliconfig.PauseValues) error {
	var (
		pauseContainers []*libpod.Container
		pauseFuncs      []shared.ParallelWorkerInput
	)
	if os.Geteuid() != 0 {
		return errors.New("pause is not supported for rootless containers")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.InputArgs
	if len(args) < 1 && !c.All {
		return errors.Errorf("you must provide at least one container name or id")
	}
	if c.All {
		containers, err := getAllOrLatestContainers(&c.PodmanCommand, runtime, libpod.ContainerStateRunning, "running")
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
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	pauseErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, pauseFuncs)
	return printParallelOutput(pauseErrors, errCount)
}
