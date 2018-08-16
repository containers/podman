package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	podRestartFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "restart all pods",
		},
		LatestPodFlag,
	}
	podRestartDescription = `Restarts one or more pods. The pod ID or name can be used.`

	podRestartCommand = cli.Command{
		Name:                   "restart",
		Usage:                  "Restart one or more pods",
		Description:            podRestartDescription,
		Flags:                  podRestartFlags,
		Action:                 podRestartCmd,
		ArgsUsage:              "POD-NAME|POD-ID [POD-NAME|POD-ID ...]",
		UseShortOptionHandling: true,
	}
)

func podRestartCmd(c *cli.Context) error {
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
