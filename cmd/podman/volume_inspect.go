package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	volumeInspectCommand     cliconfig.VolumeInspectValues
	volumeInspectDescription = `
podman volume inspect

Display detailed information on one or more volumes. Can change the format
from JSON to a Go template.
`
	_volumeInspectCommand = &cobra.Command{
		Use:   "inspect",
		Short: "Display detailed information on one or more volumes",
		Long:  volumeInspectDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			volumeInspectCommand.InputArgs = args
			volumeInspectCommand.GlobalFlags = MainGlobalOpts
			return volumeInspectCmd(&volumeInspectCommand)
		},
		Example: "[VOLUME-NAME ...]",
	}
)

func init() {
	volumeInspectCommand.Command = _volumeInspectCommand
	volumeInspectCommand.SetUsageTemplate(UsageTemplate())
	flags := volumeInspectCommand.Flags()
	flags.BoolVarP(&volumeInspectCommand.All, "all", "a", false, "Inspect all volumes")
	flags.StringVarP(&volumeInspectCommand.Format, "format", "f", "json", "Format volume output using Go template")

}

func volumeInspectCmd(c *cliconfig.VolumeInspectValues) error {
	var err error

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	opts := volumeLsOptions{
		Format: c.Format,
	}

	vols, lastError := getVolumesFromContext(&c.PodmanCommand, runtime)
	if lastError != nil {
		logrus.Errorf("%q", lastError)
	}

	return generateVolLsOutput(vols, opts, runtime)
}
