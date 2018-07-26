package main

import (
	"fmt"
	"syscall"

	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod"
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

	args := c.Args()
	var killSignal uint = uint(syscall.SIGTERM)
	var lastError error
	var pods []*libpod.Pod

	if c.String("signal") != "" {
		// Check if the signalString provided by the user is valid
		// Invalid signals will return err
		sysSignal, err := signal.ParseSignal(c.String("signal"))
		if err != nil {
			return err
		}
		killSignal = uint(sysSignal)
	}

	if c.Bool("all") {
		pods, err = runtime.Pods()
		if err != nil {
			return errors.Wrapf(err, "unable to get pods")
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
			logrus.Errorf("%q", lastError)
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to find pods %s", i)
			continue
		}
		pods = append(pods, pod)
	}

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
