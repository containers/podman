package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	podStopFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "stop all running pods",
		},
		LatestPodFlag,
	}
	podStopDescription = `
   podman pod stop

   Stops one or more running pods.  The pod name or ID can be used.
`

	podStopCommand = cli.Command{
		Name:         "stop",
		Usage:        "Stop one or more pods",
		Description:  podStopDescription,
		Flags:        podStopFlags,
		Action:       podStopCmd,
		ArgsUsage:    "POD-NAME [POD-NAME ...]",
		OnUsageError: usageErrorHandler,
	}
)

func podStopCmd(c *cli.Context) error {
	if err := checkMutuallyExclusiveFlags(c); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	// getPodsFromContext returns an error when a requested pod
	// isn't found. The only fatal error scenerio is when there are no pods
	// in which case the following loop will be skipped.
	pods, lastError := getPodsFromContext(c, runtime)

	for _, pod := range pods {
		// set cleanup to true to clean mounts and namespaces
		ctr_errs, err := pod.Stop(true)
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
