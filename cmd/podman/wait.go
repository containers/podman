package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	waitDescription = `
	podman wait

	Block until one or more containers stop and then print their exit codes
`
	waitFlags   = []cli.Flag{LatestFlag}
	waitCommand = cli.Command{
		Name:         "wait",
		Usage:        "Block on one or more containers",
		Description:  waitDescription,
		Flags:        waitFlags,
		Action:       waitCmd,
		ArgsUsage:    "CONTAINER-NAME [CONTAINER-NAME ...]",
		OnUsageError: usageErrorHandler,
	}
)

func waitCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 && !c.Bool("latest") {
		return errors.Errorf("you must provide at least one container name or id")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}

	var lastError error
	if c.Bool("latest") {
		latestCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return errors.Wrapf(err, "unable to wait on latest container")
		}
		args = append(args, latestCtr.ID())
	}

	for _, container := range args {
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
