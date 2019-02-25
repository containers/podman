package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	rmCommand     cliconfig.RmValues
	rmDescription = fmt.Sprintf(`
Podman rm will remove one or more containers from the host.
The container name or ID can be used. This does not remove images.
Running containers will not be removed without the -f option.
`)
	_rmCommand = &cobra.Command{
		Use:   "rm",
		Short: "Remove one or more containers",
		Long:  rmDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			rmCommand.InputArgs = args
			rmCommand.GlobalFlags = MainGlobalOpts
			return rmCmd(&rmCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman rm imageID
  podman rm mywebserver myflaskserver 860a4b23
  podman rm --force --all`,
	}
)

func init() {
	rmCommand.Command = _rmCommand
	rmCommand.SetUsageTemplate(UsageTemplate())
	flags := rmCommand.Flags()
	flags.BoolVarP(&rmCommand.All, "all", "a", false, "Remove all containers")
	flags.BoolVarP(&rmCommand.Force, "force", "f", false, "Force removal of a running container.  The default is false")
	flags.BoolVarP(&rmCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVarP(&rmCommand.Volumes, "volumes", "v", false, "Remove the volumes associated with the container")
	markFlagHiddenForRemoteClient("latest", flags)
}

// saveCmd saves the image to either docker-archive or oci
func rmCmd(c *cliconfig.RmValues) error {
	var (
		deleteFuncs []shared.ParallelWorkerInput
	)

	ctx := getContext()
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	failureCnt := 0
	delContainers, err := getAllOrLatestContainers(&c.PodmanCommand, runtime, -1, "all")
	if err != nil {
		if c.Force && len(c.InputArgs) > 0 {
			if errors.Cause(err) == libpod.ErrNoSuchCtr {
				err = nil
			} else {
				failureCnt++
			}
			runtime.RemoveContainersFromStorage(c.InputArgs)
		}
		if len(delContainers) == 0 {
			if err != nil && failureCnt == 0 {
				exitCode = 1
			}
			return err
		}
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	for _, container := range delContainers {
		con := container
		f := func() error {
			return runtime.RemoveContainer(ctx, con, c.Force, c.Volumes)
		}

		deleteFuncs = append(deleteFuncs, shared.ParallelWorkerInput{
			ContainerID:  con.ID(),
			ParallelFunc: f,
		})
	}
	maxWorkers := shared.Parallelize("rm")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	// Run the parallel funcs
	deleteErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, deleteFuncs)
	err = printParallelOutput(deleteErrors, errCount)
	if err != nil {
		for _, result := range deleteErrors {
			if result != nil && errors.Cause(result) != image.ErrNoSuchCtr {
				failureCnt++
			}
		}
		if failureCnt == 0 {
			exitCode = 1
		}
	}
	return err
}
