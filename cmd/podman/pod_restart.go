package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	podRestartFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "restart all pods",
		},
		LatestFlag,
	}
	podRestartDescription = `Restarts one or more pods. The pod ID or name can be used.`

	podRestartCommand = cli.Command{
		Name:                   "restart",
		Usage:                  "Restart one or more pods",
		Description:            podRestartDescription,
		Flags:                  podRestartFlags,
		Action:                 podRestartCmd,
		ArgsUsage:              "CONTAINER [CONTAINER ...]",
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

	args := c.Args()
	var pods []*libpod.Pod
	var lastError error

	if c.Bool("all") {
		pods, err = runtime.Pods()
		if err != nil {
			return errors.Wrapf(err, "unable to get running pods")
		}
	}

	if c.Bool("latest") {
		pod, err := runtime.GetLatestPod()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest pod")
		}
		pods = append(pods, pod)
	}

	for _, i := range args {
		pod, err := runtime.LookupPod(i)
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to find pod %s", i)
			continue
		}
		pods = append(pods, pod)
	}

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
