package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	waitDescription = `
	podman wait

	Block until one or more containers stop and then print their exit codes
`

	waitCommand = cli.Command{
		Name:        "wait",
		Usage:       "Block on one or more containers",
		Description: waitDescription,
		Action:      waitCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func waitCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.Errorf("you must provide at least one container name or id")
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}

	var lastError error
	for _, container := range c.Args() {
		ctr, err := runtime.LookupContainer(container)
		if err != nil {
			return errors.Wrapf(err, "unable to find container %s", container)
		}
		returnCode, err := ctr.Wait()
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to wait for the container %v", container)
		} else {
			fmt.Println(returnCode)
		}
	}

	return lastError
}
