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
	podUnpauseCommand     cliconfig.PodUnpauseValues
	podUnpauseDescription = `The podman unpause command will unpause all "paused" containers assigned to the pod.

  The pod name or ID can be used.`
	_podUnpauseCommand = &cobra.Command{
		Use:   "unpause [flags] POD [POD...]",
		Short: "Unpause one or more pods",
		Long:  podUnpauseDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podUnpauseCommand.InputArgs = args
			podUnpauseCommand.GlobalFlags = MainGlobalOpts
			podUnpauseCommand.Remote = remoteclient
			return podUnpauseCmd(&podUnpauseCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman pod unpause podID1 podID2
  podman pod unpause --all
  podman pod unpause --latest`,
	}
)

func init() {
	podUnpauseCommand.Command = _podUnpauseCommand
	podUnpauseCommand.SetHelpTemplate(HelpTemplate())
	podUnpauseCommand.SetUsageTemplate(UsageTemplate())
	flags := podUnpauseCommand.Flags()
	flags.BoolVarP(&podUnpauseCommand.All, "all", "a", false, "Unpause all running pods")
	flags.BoolVarP(&podUnpauseCommand.Latest, "latest", "l", false, "Unpause the latest pod podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

func podUnpauseCmd(c *cliconfig.PodUnpauseValues) error {
	var lastError error
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	unpauseIDs, conErrors, unpauseErrors := runtime.UnpausePods(c)

	for _, p := range unpauseIDs {
		fmt.Println(p)
	}
	if conErrors != nil && len(conErrors) > 0 {
		for ctr, err := range conErrors {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to unpause container %s", ctr)
		}
	}
	if len(unpauseErrors) > 0 {
		lastError = unpauseErrors[len(unpauseErrors)-1]
		// Remove the last error from the error slice
		unpauseErrors = unpauseErrors[:len(unpauseErrors)-1]
	}
	for _, err := range unpauseErrors {
		logrus.Errorf("%q", err)
	}
	return lastError
}
