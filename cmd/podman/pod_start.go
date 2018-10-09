package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	podStartFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "start all running pods",
		},
		LatestPodFlag,
	}
	podStartDescription = `
   podman pod start

   Starts one or more pods.  The pod name or ID can be used.
`

	podStartCommand = cli.Command{
		Name:                   "start",
		Usage:                  "Start one or more pods",
		Description:            podStartDescription,
		Flags:                  sortFlags(podStartFlags),
		Action:                 podStartCmd,
		ArgsUsage:              "POD-NAME [POD-NAME ...]",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

func podStartCmd(c *cli.Context) error {
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
