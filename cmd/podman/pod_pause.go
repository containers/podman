package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	podPauseCommand     cliconfig.PodPauseValues
	podPauseDescription = `Pauses one or more pods.  The pod name or ID can be used.`
	_podPauseCommand    = &cobra.Command{
		Use:   "pause",
		Short: "Pause one or more pods",
		Long:  podPauseDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podPauseCommand.InputArgs = args
			podPauseCommand.GlobalFlags = MainGlobalOpts
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
	podPauseCommand.SetUsageTemplate(UsageTemplate())
	flags := podPauseCommand.Flags()
	flags.BoolVarP(&podPauseCommand.All, "all", "a", false, "Pause all running pods")
	flags.BoolVarP(&podPauseCommand.Latest, "latest", "l", false, "Act on the latest pod podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

func podPauseCmd(c *cliconfig.PodPauseValues) error {
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	// getPodsFromContext returns an error when a requested pod
	// isn't found. The only fatal error scenerio is when there are no pods
	// in which case the following loop will be skipped.
	pods, lastError := getPodsFromContext(&c.PodmanCommand, runtime)

	for _, pod := range pods {
		ctr_errs, err := pod.Pause()
		if ctr_errs != nil {
			for ctr, err := range ctr_errs {
				if lastError != nil {
					logrus.Errorf("%q", lastError)
				}
				lastError = errors.Wrapf(err, "unable to pause container %q on pod %q", ctr, pod.ID())
			}
			continue
		}
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to pause pod %q", pod.ID())
			continue
		}
		fmt.Println(pod.ID())
	}

	return lastError
}
