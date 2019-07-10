package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	volumeRmCommand     cliconfig.VolumeRmValues
	volumeRmDescription = `Remove one or more existing volumes.

  By default only volumes that are not being used by any containers will be removed. To remove the volumes anyways, use the --force flag.`
	_volumeRmCommand = &cobra.Command{
		Use:     "rm [flags] VOLUME [VOLUME...]",
		Aliases: []string{"remove"},
		Short:   "Remove one or more volumes",
		Long:    volumeRmDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			volumeRmCommand.InputArgs = args
			volumeRmCommand.GlobalFlags = MainGlobalOpts
			volumeRmCommand.Remote = remoteclient
			return volumeRmCmd(&volumeRmCommand)
		},
		Example: `podman volume rm myvol1 myvol2
  podman volume rm --all
  podman volume rm --force myvol`,
	}
)

func init() {
	volumeRmCommand.Command = _volumeRmCommand
	volumeRmCommand.SetHelpTemplate(HelpTemplate())
	volumeRmCommand.SetUsageTemplate(UsageTemplate())
	flags := volumeRmCommand.Flags()
	flags.BoolVarP(&volumeRmCommand.All, "all", "a", false, "Remove all volumes")
	flags.BoolVarP(&volumeRmCommand.Force, "force", "f", false, "Remove a volume by force, even if it is being used by a container")
}

func volumeRmCmd(c *cliconfig.VolumeRmValues) error {
	var err error

	if (len(c.InputArgs) > 0 && c.All) || (len(c.InputArgs) < 1 && !c.All) {
		return errors.New("choose either one or more volumes or all")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)
	deletedVolumeNames, err := runtime.RemoveVolumes(getContext(), c)
	if err != nil {
		if len(deletedVolumeNames) > 0 {
			printDeleteVolumes(deletedVolumeNames)
			return err
		}
	}
	printDeleteVolumes(deletedVolumeNames)
	return err
}

func printDeleteVolumes(volumes []string) {
	for _, v := range volumes {
		fmt.Println(v)
	}
}
