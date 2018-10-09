package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
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

	keep := c.Bool("keep")
	args := c.Args()
	if len(args) < 1 {
		return errors.Errorf("you must provide at least one container name or id")
	}

	var lastError error
	for _, arg := range args {
		ctr, err := runtime.LookupContainer(arg)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "error looking up container %q", arg)
			continue
		}
		if err = ctr.Checkpoint(context.TODO(), keep); err != nil {
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
