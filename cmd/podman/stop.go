package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	stopCommand     cliconfig.StopValues
	stopDescription = `
   podman stop

   Stops one or more running containers.  The container name or ID can be used.
   A timeout to forcibly stop the container can also be set but defaults to 10
   seconds otherwise.
`
	_stopCommand = &cobra.Command{
		Use:   "stop",
		Short: "Stop one or more containers",
		Long:  stopDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			stopCommand.InputArgs = args
			stopCommand.GlobalFlags = MainGlobalOpts
			return stopCmd(&stopCommand)
		},
		Example: `podman stop ctrID
  podman stop --latest
  podman stop --timeout 2 mywebserver 6e534f14da9d`,
	}
)

func init() {
	stopCommand.Command = _stopCommand
	stopCommand.SetUsageTemplate(UsageTemplate())
	flags := stopCommand.Flags()
	flags.BoolVarP(&stopCommand.All, "all", "a", false, "Stop all running containers")
	flags.BoolVarP(&stopCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.UintVar(&stopCommand.Timeout, "time", libpod.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
	flags.UintVarP(&stopCommand.Timeout, "timeout", "t", libpod.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
}

func stopCmd(c *cliconfig.StopValues) error {
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "stopCmd")
		defer span.Finish()
	}

	if err := checkAllAndLatest(&c.PodmanCommand); err != nil {
		return err
	}

	rootless.SetSkipStorageSetup(true)
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	containers, err := getAllOrLatestContainers(&c.PodmanCommand, runtime, libpod.ContainerStateRunning, "running")
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
		if c.Flag("timeout").Changed {
			stopTimeout = c.Timeout
		} else {
			stopTimeout = ctr.StopTimeout()
		}
		f := func() error {
			if err := con.StopWithTimeout(stopTimeout); err != nil && errors.Cause(err) != libpod.ErrCtrStopped {
				return err
			}
			return nil

		}
		stopFuncs = append(stopFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}

	maxWorkers := shared.Parallelize("stop")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	stopErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, stopFuncs)
	return printParallelOutput(stopErrors, errCount)
}
