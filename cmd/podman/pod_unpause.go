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
	podUnpauseCommand     cliconfig.PodUnpauseValues
	podUnpauseDescription = `Unpauses one or more pods.  The pod name or ID can be used.`
	_podUnpauseCommand    = &cobra.Command{
		Use:   "unpause",
		Short: "Unpause one or more pods",
		Long:  podUnpauseDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podUnpauseCommand.InputArgs = args
			podUnpauseCommand.GlobalFlags = MainGlobalOpts
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
	podUnpauseCommand.SetUsageTemplate(UsageTemplate())
	flags := podUnpauseCommand.Flags()
	flags.BoolVarP(&podUnpauseCommand.All, "all", "a", false, "Unpause all running pods")
	flags.BoolVarP(&podUnpauseCommand.Latest, "latest", "l", false, "Unpause the latest pod podman is aware of")
}

func podUnpauseCmd(c *cliconfig.PodUnpauseValues) error {
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
		ctr_errs, err := pod.Unpause()
		if ctr_errs != nil {
			for ctr, err := range ctr_errs {
				if lastError != nil {
					logrus.Errorf("%q", lastError)
				}
				lastError = errors.Wrapf(err, "unable to unpause container %q on pod %q", ctr, pod.ID())
			}
			continue
		}
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to unpause pod %q", pod.ID())
			continue
		}
		fmt.Println(pod.ID())
	}

	return lastError
}
