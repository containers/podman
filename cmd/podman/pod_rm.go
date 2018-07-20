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
	podRmFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Remove all pods",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Force removal of a running pod by first stopping all containers, then removing all containers in the pod.  The default is false",
		},
		LatestFlag,
	}
	podRmDescription = "Remove one or more pods"
	podRmCommand     = cli.Command{
		Name: "rm",
		Usage: fmt.Sprintf(`podman rm will remove one or more pods from the host. The pod name or ID can be used.
                            A pod with containers will not be removed without --force.
                            If --force is specified, all containers will be stopped, then removed.`),
		Description:            podRmDescription,
		Flags:                  podRmFlags,
		Action:                 podRmCmd,
		ArgsUsage:              "[POD ...]",
		UseShortOptionHandling: true,
	}
)

// saveCmd saves the image to either docker-archive or oci
func podRmCmd(c *cli.Context) error {
	if err := checkMutuallyExclusiveFlags(c); err != nil {
		return err
	}
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	ctx := getContext()
	var delPods []*libpod.Pod
	var lastError error
	if c.Bool("all") {
		delPods, err = runtime.GetAllPods()
		if err != nil {
			return errors.Wrapf(err, "unable to get pod list")
		}
	}

	if c.Bool("latest") {
		delPod, err := runtime.GetLatestPod()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest pod")
		}
		delPods = append(delPods, delPod)
	}

	for _, i := range args {
		pod, err := runtime.LookupPod(i)
		if err != nil {
			logrus.Errorf("%q", lastError)
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to find pods %s", i)
			continue
		}
		delPods = append(delPods, pod)
	}
	force := c.Bool("force")

	for _, pod := range delPods {
		err = runtime.RemovePod(ctx, pod, force, force)
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "failed to delete pod %v", pod.ID())
		} else {
			fmt.Println(pod.ID())
		}
	}
	return lastError
}
