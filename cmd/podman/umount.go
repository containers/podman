package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	umountCommand cliconfig.UmountValues
	description   = `
Container storage increments a mount counter each time a container is mounted.
When a container is unmounted, the mount counter is decremented and the
container's root filesystem is physically unmounted only when the mount
counter reaches zero indicating no other processes are using the mount.
An unmount can be forced with the --force flag.
`
	_umountCommand = &cobra.Command{
		Use:     "umount",
		Aliases: []string{"unmount"},
		Short:   "Unmounts working container's root filesystem",
		Long:    description,
		RunE: func(cmd *cobra.Command, args []string) error {
			umountCommand.InputArgs = args
			umountCommand.GlobalFlags = MainGlobalOpts
			return umountCmd(&umountCommand)
		},
		Example: `podman umount ctrID
  podman umount ctrID1 ctrID2 ctrID3
  podman umount --all`,
	}
)

func init() {
	umountCommand.Command = _umountCommand
	umountCommand.SetUsageTemplate(UsageTemplate())
	flags := umountCommand.Flags()
	flags.BoolVarP(&umountCommand.All, "all", "a", false, "Umount all of the currently mounted containers")
	flags.BoolVarP(&umountCommand.Force, "force", "f", false, "Force the complete umount all of the currently mounted containers")
	flags.BoolVarP(&umountCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
}

func umountCmd(c *cliconfig.UmountValues) error {
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	force := c.Force
	umountAll := c.All
	if err := checkAllAndLatest(&c.PodmanCommand); err != nil {
		return err
	}

	containers, err := getAllOrLatestContainers(&c.PodmanCommand, runtime, -1, "all")
	if err != nil {
		if len(containers) == 0 {
			return err
		}
		fmt.Println(err.Error())
	}

	umountContainerErrStr := "error unmounting container"
	var lastError error
	for _, ctr := range containers {
		ctrState, err := ctr.State()
		if ctrState == libpod.ContainerStateRunning || err != nil {
			continue
		}

		if err = ctr.Unmount(force); err != nil {
			if umountAll && errors.Cause(err) == storage.ErrLayerNotMounted {
				continue
			}
			if lastError != nil {
				logrus.Error(lastError)
			}
			lastError = errors.Wrapf(err, "%s %s", umountContainerErrStr, ctr.ID())
			continue
		}
		fmt.Printf("%s\n", ctr.ID())
	}
	return lastError
}
