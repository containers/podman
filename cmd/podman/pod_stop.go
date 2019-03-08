package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	podStopCommand     cliconfig.PodStopValues
	podStopDescription = `The pod name or ID can be used.

  This command will stop all running containers in each of the specified pods.`

	_podStopCommand = &cobra.Command{
		Use:   "stop [flags] POD [POD...]",
		Short: "Stop one or more pods",
		Long:  podStopDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podStopCommand.InputArgs = args
			podStopCommand.GlobalFlags = MainGlobalOpts
			return podStopCmd(&podStopCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman pod stop mywebserverpod
  podman pod stop --latest
  podman pod stop --timeout 0 490eb 3557fb`,
	}
)

func init() {
	podStopCommand.Command = _podStopCommand
	podStopCommand.SetHelpTemplate(HelpTemplate())
	podStopCommand.SetUsageTemplate(UsageTemplate())
	flags := podStopCommand.Flags()
	flags.BoolVarP(&podStopCommand.All, "all", "a", false, "Stop all running pods")
	flags.BoolVarP(&podStopCommand.Latest, "latest", "l", false, "Stop the latest pod podman is aware of")
	flags.UintVarP(&podStopCommand.Timeout, "timeout", "t", 0, "Seconds to wait for pod stop before killing the container")
	markFlagHiddenForRemoteClient("latest", flags)
}

func podStopCmd(c *cliconfig.PodStopValues) error {
	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if rootless.IsRootless() {
		var err error
		c.InputArgs, c.All, c.Latest, err = joinPodNS(runtime, c.All, c.Latest, c.InputArgs)
		if err != nil {
			return err
		}
	}

	podStopIds, podStopErrors := runtime.StopPods(getContext(), c)
	for _, p := range podStopIds {
		fmt.Println(p)
	}
	if len(podStopErrors) == 0 {
		return nil
	}
	// Grab the last error
	lastError := podStopErrors[len(podStopErrors)-1]
	// Remove the last error from the error slice
	podStopErrors = podStopErrors[:len(podStopErrors)-1]

	for _, err := range podStopErrors {
		logrus.Errorf("%q", err)
	}
	return lastError
}
