package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	podRmCommand     cliconfig.PodRmValues
	podRmDescription = fmt.Sprintf(`
podman rm will remove one or more pods from the host. The pod name or ID can
be used.  A pod with containers will not be removed without --force.
If --force is specified, all containers will be stopped, then removed.
`)
	_podRmCommand = &cobra.Command{
		Use:   "rm",
		Short: "Remove one or more pods",
		Long:  podRmDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podRmCommand.InputArgs = args
			podRmCommand.GlobalFlags = MainGlobalOpts
			return podRmCmd(&podRmCommand)
		},
		Example: `podman pod rm mywebserverpod
  podman pod rm -f 860a4b23
  podman pod rm -f -a`,
	}
)

func init() {
	podRmCommand.Command = _podRmCommand
	podRmCommand.SetUsageTemplate(UsageTemplate())
	flags := podRmCommand.Flags()
	flags.BoolVarP(&podRmCommand.All, "all", "a", false, "Remove all running pods")
	flags.BoolVarP(&podRmCommand.Force, "force", "f", false, "Force removal of a running pod by first stopping all containers, then removing all containers in the pod.  The default is false")
	flags.BoolVarP(&podRmCommand.Latest, "latest", "l", false, "Remove the latest pod podman is aware of")

}

// saveCmd saves the image to either docker-archive or oci
func podRmCmd(c *cliconfig.PodRmValues) error {
	if err := checkMutuallyExclusiveFlags(&c.PodmanCommand); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	ctx := getContext()
	force := c.Force

	// getPodsFromContext returns an error when a requested pod
	// isn't found. The only fatal error scenerio is when there are no pods
	// in which case the following loop will be skipped.
	pods, lastError := getPodsFromContext(&c.PodmanCommand, runtime)

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
