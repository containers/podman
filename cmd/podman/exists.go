package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/containers/libpod/libpod/image"
	"github.com/pkg/errors"
)

var (
	imageExistsCommand     cliconfig.ImageExistsValues
	containerExistsCommand cliconfig.ContainerExistsValues
	podExistsCommand       cliconfig.PodExistsValues

	imageExistsDescription = `
	podman image exists

	Check if an image exists in local storage
`
	containerExistsDescription = `
	podman container exists

	Check if a container exists in local storage
`
	podExistsDescription = `
	podman pod exists

	Check if a pod exists in local storage
`
	_imageExistsCommand = &cobra.Command{
		Use:   "exists",
		Short: "Check if an image exists in local storage",
		Long:  imageExistsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			imageExistsCommand.InputArgs = args
			imageExistsCommand.GlobalFlags = MainGlobalOpts
			return imageExistsCmd(&imageExistsCommand)
		},
		Example: "IMAGE-NAME",
	}

	_containerExistsCommand = &cobra.Command{
		Use:   "exists",
		Short: "Check if a container exists in local storage",
		Long:  containerExistsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			containerExistsCommand.InputArgs = args
			containerExistsCommand.GlobalFlags = MainGlobalOpts
			return containerExistsCmd(&containerExistsCommand)

		},
		Example: "CONTAINER-NAME",
	}

	_podExistsCommand = &cobra.Command{
		Use:   "exists",
		Short: "Check if a pod exists in local storage",
		Long:  podExistsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podExistsCommand.InputArgs = args
			podExistsCommand.GlobalFlags = MainGlobalOpts
			return podExistsCmd(&podExistsCommand)
		},
		Example: "POD-NAME",
	}
)

func init() {
	imageExistsCommand.Command = _imageExistsCommand
	imageExistsCommand.SetUsageTemplate(UsageTemplate())
	containerExistsCommand.Command = _containerExistsCommand
	containerExistsCommand.SetUsageTemplate(UsageTemplate())
	podExistsCommand.Command = _podExistsCommand
	podExistsCommand.SetUsageTemplate(UsageTemplate())
}

func imageExistsCmd(c *cliconfig.ImageExistsValues) error {
	args := c.InputArgs
	if len(args) > 1 || len(args) < 1 {
		return errors.New("you may only check for the existence of one image at a time")
	}
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)
	if _, err := runtime.NewImageFromLocal(args[0]); err != nil {
		//TODO we need to ask about having varlink defined errors exposed
		//so we can reuse them
		if errors.Cause(err) == image.ErrNoSuchImage || err.Error() == "io.podman.ImageNotFound" {
			os.Exit(1)
		}
		return err
	}
	return nil
}

func containerExistsCmd(c *cliconfig.ContainerExistsValues) error {
	args := c.InputArgs
	if len(args) > 1 || len(args) < 1 {
		return errors.New("you may only check for the existence of one container at a time")
	}
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)
	if _, err := runtime.LookupContainer(args[0]); err != nil {
		if errors.Cause(err) == libpod.ErrNoSuchCtr || err.Error() == "io.podman.ContainerNotFound" {
			os.Exit(1)
		}
		return err
	}
	return nil
}

func podExistsCmd(c *cliconfig.PodExistsValues) error {
	args := c.InputArgs
	if len(args) > 1 || len(args) < 1 {
		return errors.New("you may only check for the existence of one pod at a time")
	}
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if _, err := runtime.LookupPod(args[0]); err != nil {
		if errors.Cause(err) == libpod.ErrNoSuchPod {
			os.Exit(1)
		}
		return err
	}
	return nil
}
