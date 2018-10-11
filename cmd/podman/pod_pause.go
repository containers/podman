package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	podPauseFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "pause all running pods",
		},
		LatestPodFlag,
	}
	podPauseDescription = `
   Pauses one or more pods.  The pod name or ID can be used.
`

	podPauseCommand = cli.Command{
		Name:                   "pause",
		Usage:                  "Pause one or more pods",
		Description:            podPauseDescription,
		Flags:                  sortFlags(podPauseFlags),
		Action:                 podPauseCmd,
		ArgsUsage:              "POD-NAME|POD-ID [POD-NAME|POD-ID ...]",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

func podPauseCmd(c *cli.Context) error {
	if err := checkMutuallyExclusiveFlags(c); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	// getPodsFromContext returns an error when a requested pod
	// isn't found. The only fatal error scenerio is when there are no pods
	// in which case the following loop will be skipped.
	pods, lastError := getPodsFromContext(c, runtime)

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
