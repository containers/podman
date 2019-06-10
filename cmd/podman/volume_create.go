package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	volumeCreateCommand     cliconfig.VolumeCreateValues
	volumeCreateDescription = `If using the default driver, "local", the volume will be created on the host in the volumes directory under container storage.`

	_volumeCreateCommand = &cobra.Command{
		Use:   "create [flags] [NAME]",
		Short: "Create a new volume",
		Long:  volumeCreateDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			volumeCreateCommand.InputArgs = args
			volumeCreateCommand.GlobalFlags = MainGlobalOpts
			volumeCreateCommand.Remote = remoteclient
			return volumeCreateCmd(&volumeCreateCommand)
		},
		Example: `podman volume create myvol
  podman volume create
  podman volume create --label foo=bar myvol`,
	}
)

func init() {
	volumeCreateCommand.Command = _volumeCreateCommand
	volumeCommand.SetHelpTemplate(HelpTemplate())
	volumeCreateCommand.SetUsageTemplate(UsageTemplate())
	flags := volumeCreateCommand.Flags()
	flags.StringVar(&volumeCreateCommand.Driver, "driver", "", "Specify volume driver name (default local)")
	flags.StringSliceVarP(&volumeCreateCommand.Label, "label", "l", []string{}, "Set metadata for a volume (default [])")
	flags.StringSliceVarP(&volumeCreateCommand.Opt, "opt", "o", []string{}, "Set driver specific options (default [])")

}

func volumeCreateCmd(c *cliconfig.VolumeCreateValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.InputArgs) > 1 {
		return errors.Errorf("too many arguments, create takes at most 1 argument")
	}

	labels, err := shared.GetAllLabels([]string{}, c.Label)
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}

	opts, err := shared.GetAllLabels([]string{}, c.Opt)
	if err != nil {
		return errors.Wrapf(err, "unable to process options")
	}

	volumeName, err := runtime.CreateVolume(getContext(), c, labels, opts)
	if err == nil {
		fmt.Println(volumeName)
	}
	return err
}
