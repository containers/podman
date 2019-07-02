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
	unpauseCommand cliconfig.UnpauseValues

	unpauseDescription = `Unpauses one or more previously paused containers.  The container name or ID can be used.`
	_unpauseCommand    = &cobra.Command{
		Use:   "unpause [flags] CONTAINER [CONTAINER...]",
		Short: "Unpause the processes in one or more containers",
		Long:  unpauseDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			unpauseCommand.InputArgs = args
			unpauseCommand.GlobalFlags = MainGlobalOpts
			unpauseCommand.Remote = remoteclient
			return unpauseCmd(&unpauseCommand)
		},
		Example: `podman unpause ctrID
  podman unpause --all`,
	}
)

func init() {
	unpauseCommand.Command = _unpauseCommand
	unpauseCommand.SetHelpTemplate(HelpTemplate())
	unpauseCommand.SetUsageTemplate(UsageTemplate())
	flags := unpauseCommand.Flags()
	flags.BoolVarP(&unpauseCommand.All, "all", "a", false, "Unpause all paused containers")
}

func unpauseCmd(c *cliconfig.UnpauseValues) error {
	if rootless.IsRootless() && !remoteclient {
		return errors.New("unpause is not supported for rootless containers")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.InputArgs
	if len(args) < 1 && !c.All {
		return errors.Errorf("you must provide at least one container name or id")
	}
	ok, failures, err := runtime.UnpauseContainers(getContext(), c)
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
