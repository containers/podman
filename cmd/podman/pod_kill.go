package main

import (
	"fmt"
	"syscall"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	podKillCommand     cliconfig.PodKillValues
	podKillDescription = `Signals are sent to the main process of each container inside the specified pod.

  The default signal is SIGKILL, or any signal specified with option --signal.`
	_podKillCommand = &cobra.Command{
		Use:   "kill [flags] POD [POD...]",
		Short: "Send the specified signal or SIGKILL to containers in pod",
		Long:  podKillDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podKillCommand.InputArgs = args
			podKillCommand.GlobalFlags = MainGlobalOpts
			podKillCommand.Remote = remoteclient
			return podKillCmd(&podKillCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman pod kill podID
  podman pod kill --signal TERM mywebserver
  podman pod kill --latest`,
	}
)

func init() {
	podKillCommand.Command = _podKillCommand
	podKillCommand.SetHelpTemplate(HelpTemplate())
	podKillCommand.SetUsageTemplate(UsageTemplate())
	flags := podKillCommand.Flags()
	flags.BoolVarP(&podKillCommand.All, "all", "a", false, "Kill all containers in all pods")
	flags.BoolVarP(&podKillCommand.Latest, "latest", "l", false, "Act on the latest pod podman is aware of")
	flags.StringVarP(&podKillCommand.Signal, "signal", "s", "KILL", "Signal to send to the containers in the pod")
	markFlagHiddenForRemoteClient("latest", flags)
}

// podKillCmd kills one or more pods with a signal
func podKillCmd(c *cliconfig.PodKillValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	killSignal := uint(syscall.SIGTERM)

	if c.Signal != "" {
		// Check if the signalString provided by the user is valid
		// Invalid signals will return err
		sysSignal, err := signal.ParseSignal(c.Signal)
		if err != nil {
			return err
		}
		killSignal = uint(sysSignal)
	}

	podKillIds, podKillErrors := runtime.KillPods(getContext(), c, killSignal)
	for _, p := range podKillIds {
		fmt.Println(p)
	}
	if len(podKillErrors) == 0 {
		return nil
	}
	// Grab the last error
	lastError := podKillErrors[len(podKillErrors)-1]
	// Remove the last error from the error slice
	podKillErrors = podKillErrors[:len(podKillErrors)-1]

	for _, err := range podKillErrors {
		logrus.Errorf("%q", err)
	}
	return lastError
}
