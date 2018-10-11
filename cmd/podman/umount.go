package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	umountFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "umount all of the currently mounted containers",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "force the complete umount all of the currently mounted containers",
		},
	}

	description = `
Container storage increments a mount counter each time a container is mounted.
When a container is unmounted, the mount counter is decremented and the
container's root filesystem is physically unmounted only when the mount
counter reaches zero indicating no other processes are using the mount.
An unmount can be forced with the --force flag.
`
	umountCommand = cli.Command{
		Name:         "umount",
		Aliases:      []string{"unmount"},
		Usage:        "Unmounts working container's root filesystem",
		Description:  description,
		Flags:        sortFlags(umountFlags),
		Action:       umountCmd,
		ArgsUsage:    "CONTAINER-NAME-OR-ID",
		OnUsageError: usageErrorHandler,
	}
)

func umountCmd(c *cli.Context) error {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	force := c.Bool("force")
	umountAll := c.Bool("all")
	args := c.Args()
	if len(args) == 0 && !umountAll {
		return errors.Errorf("container ID must be specified")
	}
	if len(args) > 0 && umountAll {
		return errors.Errorf("when using the --all switch, you may not pass any container IDs")
	}

	umountContainerErrStr := "error unmounting container"
	var lastError error
	if len(args) > 0 {
		for _, name := range args {
			ctr, err := runtime.LookupContainer(name)
			if err != nil {
				if lastError != nil {
					logrus.Error(lastError)
				}
				lastError = errors.Wrapf(err, "%s %s", umountContainerErrStr, name)
				continue
			}

			if err = ctr.Unmount(force); err != nil {
				if lastError != nil {
					logrus.Error(lastError)
				}
				lastError = errors.Wrapf(err, "%s %s", umountContainerErrStr, name)
				continue
			}
			fmt.Printf("%s\n", ctr.ID())
		}
	} else {
		containers, err := runtime.GetContainers()
		if err != nil {
			return errors.Wrapf(err, "error reading Containers")
		}
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
	}
	return lastError
}
