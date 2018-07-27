package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	podUnpauseFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "unpause all paused pods",
		},
		LatestPodFlag,
	}
	podUnpauseDescription = `
   Unpauses one or more pods.  The pod name or ID can be used.
`

	podUnpauseCommand = cli.Command{
		Name:                   "unpause",
		Usage:                  "Unpause one or more pods",
		Description:            podUnpauseDescription,
		Flags:                  podUnpauseFlags,
		Action:                 podUnpauseCmd,
		ArgsUsage:              "POD-NAME|POD-ID [POD-NAME|POD-ID ...]",
		UseShortOptionHandling: true,
	}
)

func podUnpauseCmd(c *cli.Context) error {
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
