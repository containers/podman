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
	restoreDescription = `
   podman container restore

   Restores a container from a checkpoint. The container name or ID can be used.
`
	restoreFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "keep, k",
			Usage: "keep all temporary checkpoint files",
		},
	}
	restoreCommand = cli.Command{
		Name:        "restore",
		Usage:       "Restores one or more containers from a checkpoint",
		Description: restoreDescription,
		Flags:       restoreFlags,
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
		if err = ctr.Restore(context.TODO(), keep); err != nil {
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
