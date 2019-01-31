package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	volumeCreateCommand     cliconfig.VolumeCreateValues
	volumeCreateDescription = `
podman volume create

Creates a new volume. If using the default driver, "local", the volume will
be created at.`

	_volumeCreateCommand = &cobra.Command{
		Use:   "create",
		Short: "Create a new volume",
		Long:  volumeCreateDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			volumeCreateCommand.InputArgs = args
			volumeCreateCommand.GlobalFlags = MainGlobalOpts
			return volumeCreateCmd(&volumeCreateCommand)
		},
		Example: "[VOLUME-NAME]",
	}
)

func init() {
	volumeCreateCommand.Command = _volumeCreateCommand
	flags := volumeCreateCommand.Flags()
	flags.StringVar(&volumeCreateCommand.Driver, "driver", "", "Specify volume driver name (default local)")
	flags.StringSliceVarP(&volumeCreateCommand.Label, "label", "l", []string{}, "Set metadata for a volume (default [])")
	flags.StringSliceVarP(&volumeCreateCommand.Opt, "opt", "o", []string{}, "Set driver specific options (default [])")

}

func volumeCreateCmd(c *cliconfig.VolumeCreateValues) error {
	var (
		options []libpod.VolumeCreateOption
		err     error
		volName string
	)

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.InputArgs) > 1 {
		return errors.Errorf("too many arguments, create takes at most 1 argument")
	}

	if len(c.InputArgs) > 0 {
		volName = c.InputArgs[0]
		options = append(options, libpod.WithVolumeName(volName))
	}

	if c.Flag("driver").Changed {
		options = append(options, libpod.WithVolumeDriver(c.String("driver")))
	}

	labels, err := getAllLabels([]string{}, c.Label)
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}
	if len(labels) != 0 {
		options = append(options, libpod.WithVolumeLabels(labels))
	}

	opts, err := getAllLabels([]string{}, c.Opt)
	if err != nil {
		return errors.Wrapf(err, "unable to process options")
	}
	if len(options) != 0 {
		options = append(options, libpod.WithVolumeOptions(opts))
	}

	vol, err := runtime.NewVolume(getContext(), options...)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", vol.Name())

	return nil
}
