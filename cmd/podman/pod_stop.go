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
	podStopCommand     cliconfig.PodStopValues
	podStopDescription = `
   podman pod stop

   Stops one or more running pods.  The pod name or ID can be used.
`

	_podStopCommand = &cobra.Command{
		Use:   "stop",
		Short: "Stop one or more pods",
		Long:  podStopDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podStopCommand.InputArgs = args
			podStopCommand.GlobalFlags = MainGlobalOpts
			return podStopCmd(&podStopCommand)
		},
		Example: "POD-NAME [POD-NAME ...]",
	}
)

func init() {
	podStopCommand.Command = _podStopCommand
	flags := podStopCommand.Flags()
	flags.BoolVarP(&podStopCommand.All, "all", "a", false, "Stop all running pods")
	flags.BoolVarP(&podStopCommand.Latest, "latest", "l", false, "Stop the latest pod podman is aware of")
	flags.UintVarP(&podStopCommand.Timeout, "timeout", "t", 0, "Seconds to wait for pod stop before killing the container")
}

func podStopCmd(c *cliconfig.PodStopValues) error {
	timeout := -1
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

	if c.Flag("timeout").Changed {
		timeout = int(c.Timeout)
	}
	for _, pod := range pods {
		// set cleanup to true to clean mounts and namespaces
		ctr_errs, err := pod.StopWithTimeout(ctx, true, timeout)
		if ctr_errs != nil {
			for ctr, err := range ctr_errs {
				if lastError != nil {
					logrus.Errorf("%q", lastError)
				}
				lastError = errors.Wrapf(err, "unable to stop container %q on pod %q", ctr, pod.ID())
			}
			continue
		}
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to stop pod %q", pod.ID())
			continue
		}
		fmt.Println(pod.ID())
	}
	return lastError
}
