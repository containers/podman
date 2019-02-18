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
	podRestartCommand     cliconfig.PodRestartValues
	podRestartDescription = `Restarts one or more pods. The pod ID or name can be used.`
	_podRestartCommand    = &cobra.Command{
		Use:   "restart",
		Short: "Restart one or more pods",
		Long:  podRestartDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podRestartCommand.InputArgs = args
			podRestartCommand.GlobalFlags = MainGlobalOpts
			return podRestartCmd(&podRestartCommand)
		},
		Example: `podman pod restart podID1 podID2
  podman pod restart --latest
  podman pod restart --all`,
	}
)

func init() {
	podRestartCommand.Command = _podRestartCommand
	podRestartCommand.SetUsageTemplate(UsageTemplate())
	flags := podRestartCommand.Flags()
	flags.BoolVarP(&podRestartCommand.All, "all", "a", false, "Restart all running pods")
	flags.BoolVarP(&podRestartCommand.Latest, "latest", "l", false, "Restart the latest pod podman is aware of")

}

func podRestartCmd(c *cliconfig.PodRestartValues) error {
	if err := checkMutuallyExclusiveFlags(&c.PodmanCommand); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	// getPodsFromContext returns an error when a requested pod
	// isn't found. The only fatal error scenerio is when there are no pods
	// in which case the following loop will be skipped.
	pods, lastError := getPodsFromContext(&c.PodmanCommand, runtime)

	ctx := getContext()
	for _, pod := range pods {
		ctr_errs, err := pod.Restart(ctx)
		if ctr_errs != nil {
			for ctr, err := range ctr_errs {
				if lastError != nil {
					logrus.Errorf("%q", lastError)
				}
				lastError = errors.Wrapf(err, "unable to restart container %q on pod %q", ctr, pod.ID())
			}
			continue
		}
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to restart pod %q", pod.ID())
			continue
		}
		fmt.Println(pod.ID())
	}
	return lastError
}
