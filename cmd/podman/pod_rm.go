package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
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
		LatestPodFlag,
	}
	podRmDescription = fmt.Sprintf(`
podman rm will remove one or more pods from the host. The pod name or ID can
be used.  A pod with containers will not be removed without --force.
If --force is specified, all containers will be stopped, then removed.
`)
	podRmCommand = cli.Command{
		Name:                   "rm",
		Usage:                  "Remove one or more pods",
		Description:            podRmDescription,
		Flags:                  podRmFlags,
		Action:                 podRmCmd,
		ArgsUsage:              "[POD ...]",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
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

	ctx := getContext()
	force := c.Bool("force")

	// getPodsFromContext returns an error when a requested pod
	// isn't found. The only fatal error scenerio is when there are no pods
	// in which case the following loop will be skipped.
	pods, lastError := getPodsFromContext(c, runtime)

	for _, pod := range pods {
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
