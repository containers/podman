//+build !remoteclient

package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	cleanupCommand     cliconfig.CleanupValues
	cleanupDescription = `
   podman container cleanup

   Cleans up mount points and network stacks on one or more containers from the host. The container name or ID can be used. This command is used internally when running containers, but can also be used if container cleanup has failed when a container exits.
`
	_cleanupCommand = &cobra.Command{
		Use:   "cleanup [flags] CONTAINER [CONTAINER...]",
		Short: "Cleanup network and mountpoints of one or more containers",
		Long:  cleanupDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			cleanupCommand.InputArgs = args
			cleanupCommand.GlobalFlags = MainGlobalOpts
			cleanupCommand.Remote = remoteclient
			return cleanupCmd(&cleanupCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman container cleanup --latest
  podman container cleanup ctrID1 ctrID2 ctrID3
  podman container cleanup --all`,
	}
)

func init() {
	cleanupCommand.Command = _cleanupCommand
	cleanupCommand.SetHelpTemplate(HelpTemplate())
	cleanupCommand.SetUsageTemplate(UsageTemplate())
	flags := cleanupCommand.Flags()

	flags.BoolVarP(&cleanupCommand.All, "all", "a", false, "Cleans up all containers")
	flags.BoolVarP(&cleanupCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&cleanupCommand.Remove, "rm", false, "After cleanup, remove the container entirely")
	markFlagHiddenForRemoteClient("latest", flags)
}

func cleanupCmd(c *cliconfig.CleanupValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	ok, failures, err := runtime.CleanupContainers(getContext(), c)
	if err != nil {
		return err
	}

	return printCmdResults(ok, failures)
}
