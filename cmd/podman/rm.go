package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	rmFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Remove all containers",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Force removal of a running container.  The default is false",
		},
		LatestFlag,
		cli.BoolFlag{
			Name:  "volumes, v",
			Usage: "Remove the volumes associated with the container (Not implemented yet)",
		},
	}
	rmDescription = fmt.Sprintf(`
Podman rm will remove one or more containers from the host.
The container name or ID can be used. This does not remove images.
Running containers will not be removed without the -f option.
`)
	rmCommand = cli.Command{
		Name:                   "rm",
		Usage:                  "Remove one or more containers",
		Description:            rmDescription,
		Flags:                  rmFlags,
		Action:                 rmCmd,
		ArgsUsage:              "",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

// saveCmd saves the image to either docker-archive or oci
func rmCmd(c *cli.Context) error {
	ctx := getContext()
	if err := validateFlags(c, rmFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if c.Bool("latest") && c.Bool("all") {
		return errors.Errorf("--all and --latest cannot be used together")
	}

	if len(args) == 0 && !c.Bool("all") && !c.Bool("latest") {
		return errors.Errorf("specify one or more containers to remove")
	}

	var delContainers []*libpod.Container
	var lastError error
	if c.Bool("all") {
		delContainers, err = runtime.GetContainers()
		if err != nil {
			return errors.Wrapf(err, "unable to get container list")
		}
	} else if c.Bool("latest") {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest container")
		}
		delContainers = append(delContainers, lastCtr)
	} else {
		for _, i := range args {
			container, err := runtime.LookupContainer(i)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				lastError = errors.Wrapf(err, "unable to find container %s", i)
				continue
			}
			delContainers = append(delContainers, container)
		}
	}
	for _, container := range delContainers {
		err = runtime.RemoveContainer(ctx, container, c.Bool("force"))
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to delete container %v", container.ID())
		} else {
			fmt.Println(container.ID())
		}
	}
	return lastError
}
