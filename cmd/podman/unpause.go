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
	unpauseCommand cliconfig.UnpauseValues

	unpauseDescription = `Unpauses one or more previously paused containers.  The container name or ID can be used.`
	_unpauseCommand    = &cobra.Command{
		Use:   "unpause [flags] CONTAINER [CONTAINER...]",
		Short: "Unpause the processes in one or more containers",
		Long:  unpauseDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			unpauseCommand.InputArgs = args
			unpauseCommand.GlobalFlags = MainGlobalOpts
			return unpauseCmd(&unpauseCommand)
		},
		Example: `podman unpause ctrID
  podman unpause --all`,
	}
)

func init() {
	unpauseCommand.Command = _unpauseCommand
	unpauseCommand.SetHelpTemplate(HelpTemplate())
	unpauseCommand.SetUsageTemplate(UsageTemplate())
	flags := unpauseCommand.Flags()
	flags.BoolVarP(&unpauseCommand.All, "all", "a", false, "Unpause all paused containers")
}

func unpauseCmd(c *cliconfig.UnpauseValues) error {
	var (
		unpauseContainers []*libpod.Container
		unpauseFuncs      []shared.ParallelWorkerInput
	)
	if os.Geteuid() != 0 {
		return errors.New("unpause is not supported for rootless containers")
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
		cs, err := getAllOrLatestContainers(&c.PodmanCommand, runtime, libpod.ContainerStatePaused, "paused")
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
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	unpauseErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, unpauseFuncs)
	return printParallelOutput(unpauseErrors, errCount)
}
