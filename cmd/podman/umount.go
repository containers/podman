package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	umountCommand cliconfig.UmountValues

	description = `Container storage increments a mount counter each time a container is mounted.

  When a container is unmounted, the mount counter is decremented. The container's root filesystem is physically unmounted only when the mount counter reaches zero indicating no other processes are using the mount.

  An unmount can be forced with the --force flag.
`
	_umountCommand = &cobra.Command{
		Use:     "umount [flags] CONTAINER [CONTAINER...]",
		Aliases: []string{"unmount"},
		Short:   "Unmounts working container's root filesystem",
		Long:    description,
		RunE: func(cmd *cobra.Command, args []string) error {
			umountCommand.InputArgs = args
			umountCommand.GlobalFlags = MainGlobalOpts
			return umountCmd(&umountCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman umount ctrID
  podman umount ctrID1 ctrID2 ctrID3
  podman umount --all`,
	}
)

func init() {
	umountCommand.Command = _umountCommand
	umountCommand.SetHelpTemplate(HelpTemplate())
	umountCommand.SetUsageTemplate(UsageTemplate())
	flags := umountCommand.Flags()
	flags.BoolVarP(&umountCommand.All, "all", "a", false, "Umount all of the currently mounted containers")
	flags.BoolVarP(&umountCommand.Force, "force", "f", false, "Force the complete umount all of the currently mounted containers")
	flags.BoolVarP(&umountCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

func umountCmd(c *cliconfig.UmountValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating runtime")
	}
	defer runtime.Shutdown(false)

	ok, failures, err := runtime.UmountRootFilesystems(getContext(), c)
	if err != nil {
		return err
	}
	return printCmdResults(ok, failures)
}
