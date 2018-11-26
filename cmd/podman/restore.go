package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	restoreDescription = `
   podman container restore

   Restores a container from a checkpoint. The container name or ID can be used.
`
	restoreFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "keep, k",
			Usage: "keep all temporary checkpoint files",
		},
		// restore --all would make more sense if there would be
		// dedicated state for container which are checkpointed.
		// TODO: add ContainerStateCheckpointed
		cli.BoolFlag{
			Name:  "tcp-established",
			Usage: "checkpoint a container with established TCP connections",
		},
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "restore all checkpointed containers",
		},
		LatestFlag,
	}
	restoreCommand = cli.Command{
		Name:        "restore",
		Usage:       "Restores one or more containers from a checkpoint",
		Description: restoreDescription,
		Flags:       sortFlags(restoreFlags),
		Action:      restoreCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func restoreCmd(c *cli.Context) error {
	if rootless.IsRootless() {
		return errors.New("restoring a container requires root")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	options := libpod.ContainerCheckpointOptions{
		Keep:           c.Bool("keep"),
		TCPEstablished: c.Bool("tcp-established"),
	}

	if err := checkAllAndLatest(c); err != nil {
		return err
	}

	containers, lastError := getAllOrLatestContainers(c, runtime, libpod.ContainerStateExited, "checkpointed")

	for _, ctr := range containers {
		if err = ctr.Restore(context.TODO(), options); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to restore container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
