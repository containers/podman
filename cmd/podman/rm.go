package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	rmCommand     cliconfig.RmValues
	rmDescription = fmt.Sprintf(`Removes one or more containers from the host. The container name or ID can be used.

  Command does not remove images. Running containers will not be removed without the -f option.`)
	_rmCommand = &cobra.Command{
		Use:   "rm [flags] CONTAINER [CONTAINER...]",
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
	rmCommand.SetHelpTemplate(HelpTemplate())
	rmCommand.SetUsageTemplate(UsageTemplate())
	flags := rmCommand.Flags()
	flags.BoolVarP(&rmCommand.All, "all", "a", false, "Remove all containers")
	flags.BoolVarP(&rmCommand.Force, "force", "f", false, "Force removal of a running container.  The default is false")
	flags.BoolVarP(&rmCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVarP(&rmCommand.Volumes, "volumes", "v", false, "Remove the volumes associated with the container")
	markFlagHiddenForRemoteClient("latest", flags)
}

func joinContainerOrCreateRootlessUserNS(runtime *libpod.Runtime, ctr *libpod.Container) (bool, int, error) {
	if os.Geteuid() == 0 {
		return false, 0, nil
	}
	s, err := ctr.State()
	if err != nil {
		return false, -1, err
	}
	opts := rootless.Opts{
		Argument: ctr.ID(),
	}
	if s == libpod.ContainerStateRunning || s == libpod.ContainerStatePaused {
		data, err := ioutil.ReadFile(ctr.Config().ConmonPidFile)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot read conmon PID file %q", ctr.Config().ConmonPidFile)
		}
		conmonPid, err := strconv.Atoi(string(data))
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot parse PID %q", data)
		}
		return rootless.JoinDirectUserAndMountNSWithOpts(uint(conmonPid), &opts)
	}
	return rootless.BecomeRootInUserNSWithOpts(&opts)
}

// saveCmd saves the image to either docker-archive or oci
func rmCmd(c *cliconfig.RmValues) error {
	var (
		deleteFuncs []shared.ParallelWorkerInput
	)
	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	ctx := getContext()
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if rootless.IsRootless() {
		// When running in rootless mode we cannot manage different containers and
		// user namespaces from the same context, so be sure to re-exec once for each
		// container we are dealing with.
		// What we do is to first collect all the containers we want to delete, then
		// we re-exec in each of the container namespaces and from there remove the single
		// container.
		var container *libpod.Container
		if os.Geteuid() == 0 {
			// We are in the namespace, override InputArgs with the single
			// argument that was passed down to us.
			c.All = false
			c.Latest = false
			c.InputArgs = []string{rootless.Argument()}
		} else {
			var containers []*libpod.Container
			if c.All {
				containers, err = runtime.GetContainers()
			} else if c.Latest {
				container, err = runtime.GetLatestContainer()
				if err != nil {
					return errors.Wrapf(err, "unable to get latest pod")
				}
				containers = append(containers, container)
			} else {
				for _, c := range c.InputArgs {
					container, err = runtime.LookupContainer(c)
					if err != nil {
						return err
					}
					containers = append(containers, container)
				}
			}
			// Now we really delete the containers.
			for _, c := range containers {
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
	}

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
			if errors.Cause(err) == libpod.ErrNoSuchCtr {
				exitCode = 1
			}
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

	if failureCnt > 0 {
		exitCode = 125
	}

	return err
}
