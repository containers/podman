package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	restartCommand     cliconfig.RestartValues
	restartDescription = `Restarts one or more running containers. The container ID or name can be used.

  A timeout before forcibly stopping can be set, but defaults to 10 seconds.`
	_restartCommand = &cobra.Command{
		Use:   "restart [flags] CONTAINER [CONTAINER...]",
		Short: "Restart one or more containers",
		Long:  restartDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			restartCommand.InputArgs = args
			restartCommand.GlobalFlags = MainGlobalOpts
			return restartCmd(&restartCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman restart ctrID
  podman restart --latest
  podman restart ctrID1 ctrID2`,
	}
)

func init() {
	restartCommand.Command = _restartCommand
	restartCommand.SetHelpTemplate(HelpTemplate())
	restartCommand.SetUsageTemplate(UsageTemplate())
	flags := restartCommand.Flags()
	flags.BoolVarP(&restartCommand.All, "all", "a", false, "Restart all non-running containers")
	flags.BoolVarP(&restartCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&restartCommand.Running, "running", false, "Restart only running containers when --all is used")
	flags.UintVarP(&restartCommand.Timeout, "timeout", "t", define.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
	flags.UintVar(&restartCommand.Timeout, "time", define.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")

	markFlagHiddenForRemoteClient("latest", flags)
}

func restartCmd(c *cliconfig.RestartValues) error {
	all := c.All
	if len(c.InputArgs) < 1 && !c.Latest && !all {
		return errors.Wrapf(define.ErrInvalidArg, "you must provide at least one container name or ID")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	ok, failures, err := runtime.Restart(getContext(), c)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			if len(c.InputArgs) > 1 {
				exitCode = define.ExecErrorCodeGeneric
			} else {
				exitCode = 1
			}
		}
		return err
	}
	if len(failures) > 0 {
		exitCode = define.ExecErrorCodeGeneric
	}
	return printCmdResults(ok, failures)
}
