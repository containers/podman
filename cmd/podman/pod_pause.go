package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	podPauseCommand     cliconfig.PodPauseValues
	podPauseDescription = `The pod name or ID can be used.

  All running containers within each specified pod will then be paused.`
	_podPauseCommand = &cobra.Command{
		Use:   "pause [flags] POD [POD...]",
		Short: "Pause one or more pods",
		Long:  podPauseDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podPauseCommand.InputArgs = args
			podPauseCommand.GlobalFlags = MainGlobalOpts
			podPauseCommand.Remote = remoteclient
			return podPauseCmd(&podPauseCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman pod pause podID1 podID2
  podman pod pause --latest
  podman pod pause --all`,
	}
)

func init() {
	podPauseCommand.Command = _podPauseCommand
	podPauseCommand.SetHelpTemplate(HelpTemplate())
	podPauseCommand.SetUsageTemplate(UsageTemplate())
	flags := podPauseCommand.Flags()
	flags.BoolVarP(&podPauseCommand.All, "all", "a", false, "Pause all running pods")
	flags.BoolVarP(&podPauseCommand.Latest, "latest", "l", false, "Act on the latest pod podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

func podPauseCmd(c *cliconfig.PodPauseValues) error {
	var lastError error
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	pauseIDs, conErrors, pauseErrors := runtime.PausePods(c)

	for _, p := range pauseIDs {
		fmt.Println(p)
	}
	if conErrors != nil && len(conErrors) > 0 {
		for ctr, err := range conErrors {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to pause container %s", ctr)
		}
	}
	if len(pauseErrors) > 0 {
		lastError = pauseErrors[len(pauseErrors)-1]
		// Remove the last error from the error slice
		pauseErrors = pauseErrors[:len(pauseErrors)-1]
	}
	for _, err := range pauseErrors {
		logrus.Errorf("%q", err)
	}
	return lastError
}
