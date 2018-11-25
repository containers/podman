package main

import (
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	imageExistsDescription = `
	podman image exists

	Check if an image exists in local storage
`

	imageExistsCommand = cli.Command{
		Name:         "exists",
		Usage:        "Check if an image exists in local storage",
		Description:  imageExistsDescription,
		Action:       imageExistsCmd,
		ArgsUsage:    "IMAGE-NAME",
		OnUsageError: usageErrorHandler,
	}
)

var (
	containerExistsDescription = `
	podman container exists

	Check if a container exists in local storage
`

	containerExistsCommand = cli.Command{
		Name:         "exists",
		Usage:        "Check if a container exists in local storage",
		Description:  containerExistsDescription,
		Action:       containerExistsCmd,
		ArgsUsage:    "CONTAINER-NAME",
		OnUsageError: usageErrorHandler,
	}
)

func imageExistsCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) > 1 || len(args) < 1 {
		return errors.New("you may only check for the existence of one image at a time")
	}
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)
	if _, err := runtime.ImageRuntime().NewFromLocal(args[0]); err != nil {
		if errors.Cause(err) == image.ErrNoSuchImage {
			os.Exit(1)
		}
		return err
	}
	return nil
}

func containerExistsCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) > 1 || len(args) < 1 {
		return errors.New("you may only check for the existence of one container at a time")
	}
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)
	if _, err := runtime.LookupContainer(args[0]); err != nil {
		if errors.Cause(err) == libpod.ErrNoSuchCtr {
			os.Exit(1)
		}
		return err
	}
	return nil
}
