package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	pauseCommand     cliconfig.PauseValues
	pauseDescription = `Pauses one or more running containers.  The container name or ID can be used.`
	_pauseCommand    = &cobra.Command{
		Use:   "pause [flags] CONTAINER [CONTAINER...]",
		Short: "Pause all the processes in one or more containers",
		Long:  pauseDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pauseCommand.InputArgs = args
			pauseCommand.GlobalFlags = MainGlobalOpts
			pauseCommand.Remote = remoteclient
			return pauseCmd(&pauseCommand)
		},
		Example: `podman pause mywebserver
  podman pause 860a4b23
  podman pause -a`,
	}
)

func init() {
	pauseCommand.Command = _pauseCommand
	pauseCommand.SetHelpTemplate(HelpTemplate())
	pauseCommand.SetUsageTemplate(UsageTemplate())
	flags := pauseCommand.Flags()
	flags.BoolVarP(&pauseCommand.All, "all", "a", false, "Pause all running containers")

}

func pauseCmd(c *cliconfig.PauseValues) error {
	if rootless.IsRootless() && !remoteclient {
		return errors.New("pause is not supported for rootless containers")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	args := c.InputArgs
	if len(args) < 1 && !c.All {
		return errors.Errorf("you must provide at least one container name or id")
	}
	ok, failures, err := runtime.PauseContainers(getContext(), c)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			if len(c.InputArgs) > 1 {
				exitCode = 125
			} else {
				exitCode = 1
			}
		}
		return err
	}
	if len(failures) > 0 {
		exitCode = 125
	}
	return printCmdResults(ok, failures)
}
