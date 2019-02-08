package main

import (
	"encoding/json"
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	podInspectCommand     cliconfig.PodInspectValues
	podInspectDescription = "Display the configuration for a pod by name or id"
	_podInspectCommand    = &cobra.Command{
		Use:   "inspect",
		Short: "Displays a pod configuration",
		Long:  podInspectDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podInspectCommand.InputArgs = args
			podInspectCommand.GlobalFlags = MainGlobalOpts
			return podInspectCmd(&podInspectCommand)
		},
		Example: "[POD_NAME_OR_ID]",
	}
)

func init() {
	podInspectCommand.Command = _podInspectCommand
	flags := podInspectCommand.Flags()
	flags.BoolVarP(&podInspectCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")

}

func podInspectCmd(c *cliconfig.PodInspectValues) error {
	var (
		pod *libpod.Pod
	)
	if err := checkMutuallyExclusiveFlags(&c.PodmanCommand); err != nil {
		return err
	}
	args := c.InputArgs
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if c.Latest {
		pod, err = runtime.GetLatestPod()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest pod")
		}
	} else {
		pod, err = runtime.LookupPod(args[0])
		if err != nil {
			return err
		}
	}

	podInspectData, err := pod.Inspect()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(&podInspectData, "", "     ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
