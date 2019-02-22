package main

import (
	"fmt"
	"syscall"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	podKillCommand     cliconfig.PodKillValues
	podKillDescription = "The main process of each container inside the specified pod will be sent SIGKILL, or any signal specified with option --signal."
	_podKillCommand    = &cobra.Command{
		Use:   "kill",
		Short: "Send the specified signal or SIGKILL to containers in pod",
		Long:  podKillDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podKillCommand.InputArgs = args
			podKillCommand.GlobalFlags = MainGlobalOpts
			return podKillCmd(&podKillCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman pod kill podID
  podman pod kill --signal TERM mywebserver
  podman pod kill --latest`,
	}
)

func init() {
	podKillCommand.Command = _podKillCommand
	podKillCommand.SetUsageTemplate(UsageTemplate())
	flags := podKillCommand.Flags()
	flags.BoolVarP(&podKillCommand.All, "all", "a", false, "Kill all containers in all pods")
	flags.BoolVarP(&podKillCommand.Latest, "latest", "l", false, "Act on the latest pod podman is aware of")
	flags.StringVarP(&podKillCommand.Signal, "signal", "s", "KILL", "Signal to send to the containers in the pod")
	markFlagHiddenForRemoteClient("latest", flags)
}

// podKillCmd kills one or more pods with a signal
func podKillCmd(c *cliconfig.PodKillValues) error {
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var killSignal uint = uint(syscall.SIGTERM)

	if c.Signal != "" {
		// Check if the signalString provided by the user is valid
		// Invalid signals will return err
		sysSignal, err := signal.ParseSignal(c.Signal)
		if err != nil {
			return err
		}
		killSignal = uint(sysSignal)
	}

	// getPodsFromContext returns an error when a requested pod
	// isn't found. The only fatal error scenerio is when there are no pods
	// in which case the following loop will be skipped.
	pods, lastError := getPodsFromContext(&c.PodmanCommand, runtime)

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
