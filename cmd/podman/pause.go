package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	pauseDescription = `
   podman pause

   Pauses one or more running containers.  The container name or ID can be used.
`
	pauseCommand = cli.Command{
		Name:        "pause",
		Usage:       "Pauses all the processes in one or more containers",
		Description: pauseDescription,
		Action:      pauseCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func pauseCmd(c *cli.Context) error {
	if os.Getuid() != 0 {
		return errors.New("pause is not supported for rootless containers")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

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
		if err = ctr.Pause(); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to pause container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
