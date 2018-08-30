package main

import (
	"fmt"
	"syscall"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	podKillFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Kill all containers in all pods",
		},
		cli.StringFlag{
			Name:  "signal, s",
			Usage: "Signal to send to the containers in the pod",
			Value: "KILL",
		},
		LatestPodFlag,
	}
	podKillDescription = "The main process of each container inside the specified pod will be sent SIGKILL, or any signal specified with option --signal."
	podKillCommand     = cli.Command{
		Name:                   "kill",
		Usage:                  "Send the specified signal or SIGKILL to containers in pod",
		Description:            podKillDescription,
		Flags:                  podKillFlags,
		Action:                 podKillCmd,
		ArgsUsage:              "[POD_NAME_OR_ID]",
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

// podKillCmd kills one or more pods with a signal
func podKillCmd(c *cli.Context) error {
	if err := checkMutuallyExclusiveFlags(c); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var killSignal uint = uint(syscall.SIGTERM)

	if c.String("signal") != "" {
		// Check if the signalString provided by the user is valid
		// Invalid signals will return err
		sysSignal, err := signal.ParseSignal(c.String("signal"))
		if err != nil {
			return err
		}
		killSignal = uint(sysSignal)
	}

	// getPodsFromContext returns an error when a requested pod
	// isn't found. The only fatal error scenerio is when there are no pods
	// in which case the following loop will be skipped.
	pods, lastError := getPodsFromContext(c, runtime)

	for _, pod := range pods {
		ctr_errs, err := pod.Kill(killSignal)
		if ctr_errs != nil {
			for ctr, err := range ctr_errs {
				if lastError != nil {
					logrus.Errorf("%q", lastError)
				}
				lastError = errors.Wrapf(err, "unable to kill container %q in pod %q", ctr, pod.ID())
			}
			continue
		}
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to kill pod %q", pod.ID())
			continue
		}
		fmt.Println(pod.ID())
	}
	return lastError
}
