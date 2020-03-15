package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	podInspectCommand     cliconfig.PodInspectValues
	podInspectDescription = `Display the configuration for a pod by name or id

  By default, this will render all results in a JSON array.`

	_podInspectCommand = &cobra.Command{
		Use:   "inspect [flags] POD",
		Short: "Displays a pod configuration",
		Long:  podInspectDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podInspectCommand.InputArgs = args
			podInspectCommand.GlobalFlags = MainGlobalOpts
			podInspectCommand.Remote = remoteclient
			return podInspectCmd(&podInspectCommand)
		},
		Example: `podman pod inspect podID`,
	}
)

func init() {
	podInspectCommand.Command = _podInspectCommand
	podInspectCommand.SetHelpTemplate(HelpTemplate())
	podInspectCommand.SetUsageTemplate(UsageTemplate())
	flags := podInspectCommand.Flags()
	flags.BoolVarP(&podInspectCommand.Latest, "latest", "l", false, "Act on the latest pod podman is aware of")

	markFlagHiddenForRemoteClient("latest", flags)
}

func podInspectCmd(c *cliconfig.PodInspectValues) error {
	var (
		pod *adapter.Pod
	)
	args := c.InputArgs

	if len(args) < 1 && !c.Latest {
		return errors.Errorf("you must provide the name or id of a pod")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

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
