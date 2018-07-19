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
	podStopFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "stop all running pods",
		},
		LatestFlag,
	}
	podStopDescription = `
   podman pod stop

   Stops one or more running pods.  The pod name or ID can be used.
`

	podStopCommand = cli.Command{
		Name:        "stop",
		Usage:       "Stop one or more pods",
		Description: podStopDescription,
		Flags:       podStopFlags,
		Action:      podStopCmd,
		ArgsUsage:   "POD-NAME [POD-NAME ...]",
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

	for _, pod := range pods {
		// set cleanup to true to clean mounts and namespaces
		ctr_errs, err := pod.Stop(true)
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to stop pod %q", pod.ID())
			continue
		} else if ctr_errs != nil {
			for ctr, err := range ctr_errs {
				if lastError != nil {
					logrus.Errorf("%q", lastError)
				}
				lastError = errors.Wrapf(err, "unable to stop container %q on pod %q", ctr, pod.ID())
			}
			continue
		}
		fmt.Println(pod.ID())
	}
	return lastError
}
