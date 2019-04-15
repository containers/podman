package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	stopCommand     cliconfig.StopValues
	stopDescription = `Stops one or more running containers.  The container name or ID can be used.

  A timeout to forcibly stop the container can also be set but defaults to 10 seconds otherwise.`
	_stopCommand = &cobra.Command{
		Use:   "stop [flags] CONTAINER [CONTAINER...]",
		Short: "Stop one or more containers",
		Long:  stopDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			stopCommand.InputArgs = args
			stopCommand.GlobalFlags = MainGlobalOpts
			return stopCmd(&stopCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman stop ctrID
  podman stop --latest
  podman stop --timeout 2 mywebserver 6e534f14da9d`,
	}
)

func init() {
	stopCommand.Command = _stopCommand
	stopCommand.SetHelpTemplate(HelpTemplate())
	stopCommand.SetUsageTemplate(UsageTemplate())
	flags := stopCommand.Flags()
	flags.BoolVarP(&stopCommand.All, "all", "a", false, "Stop all running containers")
	flags.BoolVarP(&stopCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.UintVar(&stopCommand.Timeout, "time", libpod.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
	flags.UintVarP(&stopCommand.Timeout, "timeout", "t", libpod.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
	markFlagHiddenForRemoteClient("latest", flags)
}

// stopCmd stops a container or containers
func stopCmd(c *cliconfig.StopValues) error {
	if c.Flag("timeout").Changed && c.Flag("time").Changed {
		return errors.New("the --timeout and --time flags are mutually exclusive")
	}

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	ok, failures, err := runtime.StopContainers(getContext(), c)
	if err != nil {
		return err
	}
	return printCmdResults(ok, failures)
}
