package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/adapter"
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
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.InputArgs) > 1 {
		return errors.Errorf("too many arguments, create takes at most 1 argument")
	}

	labels, err := getAllLabels([]string{}, c.Label)
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}

	opts, err := getAllLabels([]string{}, c.Opt)
	if err != nil {
		return errors.Wrapf(err, "unable to process options")
	}

	volumeName, err := runtime.CreateVolume(getContext(), c, labels, opts)
	if err == nil {
		fmt.Println(volumeName)
	}
	return err
}
