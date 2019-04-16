package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
	"os"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
)

var (
	imageExistsCommand     cliconfig.ImageExistsValues
	containerExistsCommand cliconfig.ContainerExistsValues
	podExistsCommand       cliconfig.PodExistsValues

	imageExistsDescription = `If the named image exists in local storage, podman image exists exits with 0, otherwise the exit code will be 1.`

	containerExistsDescription = `If the named container exists in local storage, podman container exists exits with 0, otherwise the exit code will be 1.`

	podExistsDescription = `If the named pod exists in local storage, podman pod exists exits with 0, otherwise the exit code will be 1.`

	_imageExistsCommand = &cobra.Command{
		Use:   "exists IMAGE",
		Short: "Check if an image exists in local storage",
		Long:  imageExistsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			imageExistsCommand.InputArgs = args
			imageExistsCommand.GlobalFlags = MainGlobalOpts
			imageExistsCommand.Remote = remoteclient
			return imageExistsCmd(&imageExistsCommand)
		},
		Example: `podman image exists imageID
  podman image exists alpine || podman pull alpine`,
	}

	_containerExistsCommand = &cobra.Command{
		Use:   "exists CONTAINER",
		Short: "Check if a container exists in local storage",
		Long:  containerExistsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			containerExistsCommand.InputArgs = args
			containerExistsCommand.GlobalFlags = MainGlobalOpts
			containerExistsCommand.Remote = remoteclient
			return containerExistsCmd(&containerExistsCommand)

		},
		Example: `podman container exists containerID
  podman container exists myctr || podman run --name myctr [etc...]`,
	}

	_podExistsCommand = &cobra.Command{
		Use:   "exists POD",
		Short: "Check if a pod exists in local storage",
		Long:  podExistsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podExistsCommand.InputArgs = args
			podExistsCommand.GlobalFlags = MainGlobalOpts
			podExistsCommand.Remote = remoteclient
			return podExistsCmd(&podExistsCommand)
		},
		Example: `podman pod exists podID
  podman pod exists mypod || podman pod create --name mypod`,
	}
)

func init() {
	imageExistsCommand.Command = _imageExistsCommand
	imageExistsCommand.DisableFlagsInUseLine = true
	imageExistsCommand.SetHelpTemplate(HelpTemplate())
	imageExistsCommand.SetUsageTemplate(UsageTemplate())
	containerExistsCommand.Command = _containerExistsCommand
	containerExistsCommand.DisableFlagsInUseLine = true
	containerExistsCommand.SetHelpTemplate(HelpTemplate())
	containerExistsCommand.SetUsageTemplate(UsageTemplate())
	podExistsCommand.Command = _podExistsCommand
	podExistsCommand.DisableFlagsInUseLine = true
	podExistsCommand.SetHelpTemplate(HelpTemplate())
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
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if _, err := runtime.LookupPod(args[0]); err != nil {
		if errors.Cause(err) == libpod.ErrNoSuchPod || err.Error() == "io.podman.PodNotFound" {
			os.Exit(1)
		}
		return err
	}
	return nil
}
