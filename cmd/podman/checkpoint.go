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
	checkpointDescription = `
   podman container checkpoint

   Checkpoints one or more running containers. The container name or ID can be used.
`
	checkpointFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "keep, k",
			Usage: "keep all temporary checkpoint files",
		},
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "checkpoint all running containers",
		},
		LatestFlag,
	}
	checkpointCommand = cli.Command{
		Name:        "checkpoint",
		Usage:       "Checkpoints one or more containers",
		Description: checkpointDescription,
		Flags:       sortFlags(checkpointFlags),
		Action:      checkpointCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func checkpointCmd(c *cli.Context) error {
	if rootless.IsRootless() {
		return errors.New("checkpointing a container requires root")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	options := libpod.ContainerCheckpointOptions{
		Keep: c.Bool("keep"),
	}

	if err := checkAllAndLatest(c); err != nil {
		return err
	}

	containers, lastError := getAllOrLatestContainers(c, runtime, libpod.ContainerStateRunning, "running")

	for _, ctr := range containers {
		if err = ctr.Checkpoint(context.TODO(), options); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to checkpoint container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
