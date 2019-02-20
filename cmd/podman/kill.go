package main

import (
	"fmt"
	"syscall"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	killCommand cliconfig.KillValues

	killDescription = "The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal."
	_killCommand    = &cobra.Command{
		Use:   "kill",
		Short: "Kill one or more running containers with a specific signal",
		Long:  killDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			killCommand.InputArgs = args
			killCommand.GlobalFlags = MainGlobalOpts
			return killCmd(&killCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman kill mywebserver
  podman kill 860a4b23
  podman kill --signal TERM ctrID`,
	}
)

func init() {
	killCommand.Command = _killCommand
	killCommand.SetUsageTemplate(UsageTemplate())
	flags := killCommand.Flags()

	flags.BoolVarP(&killCommand.All, "all", "a", false, "Signal all running containers")
	flags.StringVarP(&killCommand.Signal, "signal", "s", "KILL", "Signal to send to the container")
	flags.BoolVarP(&killCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")

}

// killCmd kills one or more containers with a signal
func killCmd(c *cliconfig.KillValues) error {
	var (
		killFuncs  []shared.ParallelWorkerInput
		killSignal uint = uint(syscall.SIGTERM)
	)

	rootless.SetSkipStorageSetup(true)
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if c.Signal != "" {
		// Check if the signalString provided by the user is valid
		// Invalid signals will return err
		sysSignal, err := signal.ParseSignal(c.Signal)
		if err != nil {
			return err
		}
		killSignal = uint(sysSignal)
	}

	containers, err := getAllOrLatestContainers(&c.PodmanCommand, runtime, libpod.ContainerStateRunning, "running")
	if err != nil {
		if len(containers) == 0 {
			return err
		}
		fmt.Println(err.Error())
	}

	for _, ctr := range containers {
		con := ctr
		f := func() error {
			return con.Kill(killSignal)
		}

		killFuncs = append(killFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}

	maxWorkers := shared.Parallelize("kill")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	killErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, killFuncs)
	return printParallelOutput(killErrors, errCount)
}
