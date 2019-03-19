package main

import (
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
	restartDescription = `Restarts one or more running containers. The container ID or name can be used.

  A timeout before forcibly stopping can be set, but defaults to 10 seconds.`
	_restartCommand = &cobra.Command{
		Use:   "restart [flags] CONTAINER [CONTAINER...]",
		Short: "Restart one or more containers",
		Long:  restartDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			restartCommand.InputArgs = args
			restartCommand.GlobalFlags = MainGlobalOpts
			return restartCmd(&restartCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman restart ctrID
  podman restart --latest
  podman restart ctrID1 ctrID2`,
	}
)

func init() {
	restartCommand.Command = _restartCommand
	restartCommand.SetHelpTemplate(HelpTemplate())
	restartCommand.SetUsageTemplate(UsageTemplate())
	flags := restartCommand.Flags()
	flags.BoolVarP(&restartCommand.All, "all", "a", false, "Restart all non-running containers")
	flags.BoolVarP(&restartCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&restartCommand.Running, "running", false, "Restart only running containers when --all is used")
	flags.UintVarP(&restartCommand.Timeout, "timeout", "t", libpod.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
	flags.UintVar(&restartCommand.Timeout, "time", libpod.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")

	markFlagHiddenForRemoteClient("latest", flags)
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
	if rootless.IsRootless() {
		// If we are in the re-execed rootless environment,
		// override the arg to deal only with one container.
		if os.Geteuid() == 0 {
			c.All = false
			c.Latest = false
			c.InputArgs = []string{rootless.Argument()}
		}
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
	useTimeout := c.Flag("timeout").Changed || c.Flag("time").Changed

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

	if os.Geteuid() != 0 {
		// In rootless mode we can deal with one container at at time.
		for _, c := range restartContainers {
			_, ret, err := joinContainerOrCreateRootlessUserNS(runtime, c)
			if err != nil {
				return err
			}
			if ret != 0 {
				os.Exit(ret)
			}
		}
		os.Exit(0)
	}

	maxWorkers := shared.Parallelize("restart")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}

	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

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
