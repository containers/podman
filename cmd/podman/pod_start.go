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
	podStartCommand     cliconfig.PodStartValues
	podStartDescription = `
   podman pod start

   Starts one or more pods.  The pod name or ID can be used.
`
	_podStartCommand = &cobra.Command{
		Use:   "start",
		Short: "Start one or more pods",
		Long:  podStartDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podStartCommand.InputArgs = args
			podStartCommand.GlobalFlags = MainGlobalOpts
			return podStartCmd(&podStartCommand)
		},
		Example: "POD-NAME [POD-NAME ...]",
	}
)

func init() {
	podStartCommand.Command = _podStartCommand
	podStartCommand.SetUsageTemplate(UsageTemplate())
	flags := podStartCommand.Flags()
	flags.BoolVarP(&podStartCommand.All, "all", "a", false, "Start all pods")
	flags.BoolVarP(&podStartCommand.Latest, "latest", "l", false, "Start the latest pod podman is aware of")
}

func podStartCmd(c *cliconfig.PodStartValues) error {
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
		ctr_errs, err := pod.Start(ctx)
		if ctr_errs != nil {
			for ctr, err := range ctr_errs {
				if lastError != nil {
					logrus.Errorf("%q", lastError)
				}
				lastError = errors.Wrapf(err, "unable to start container %q on pod %q", ctr, pod.ID())
			}
			continue
		}
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to start pod %q", pod.ID())
			continue
		}
		fmt.Println(pod.ID())
	}

	return lastError
}
