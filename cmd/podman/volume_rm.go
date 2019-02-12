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
	volumeRmCommand     cliconfig.VolumeRmValues
	volumeRmDescription = `
podman volume rm

Remove one or more existing volumes. Will only remove volumes that are
not being used by any containers. To remove the volumes anyways, use the
--force flag.
`
	_volumeRmCommand = &cobra.Command{
		Use:     "rm",
		Aliases: []string{"remove"},
		Short:   "Remove one or more volumes",
		Long:    volumeRmDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			volumeRmCommand.InputArgs = args
			volumeRmCommand.GlobalFlags = MainGlobalOpts
			return volumeRmCmd(&volumeRmCommand)
		},
		Example: "[VOLUME-NAME ...]",
	}
)

func init() {
	volumeRmCommand.Command = _volumeRmCommand
	volumeRmCommand.SetUsageTemplate(UsageTemplate())
	flags := volumeRmCommand.Flags()
	flags.BoolVarP(&volumeRmCommand.All, "all", "a", false, "Remove all volumes")
	flags.BoolVarP(&volumeRmCommand.Force, "force", "f", false, "Remove a volume by force, even if it is being used by a container")
}

func volumeRmCmd(c *cliconfig.VolumeRmValues) error {
	var err error

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	ctx := getContext()

	vols, lastError := getVolumesFromContext(&c.PodmanCommand, runtime)
	for _, vol := range vols {
		err = runtime.RemoveVolume(ctx, vol, c.Force, false)
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "failed to remove volume %q", vol.Name())
		} else {
			fmt.Println(vol.Name())
		}
	}
	return lastError
}
