package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	volumeInspectCommand     cliconfig.VolumeInspectValues
	volumeInspectDescription = `Display detailed information on one or more volumes.

  Use a Go template to change the format from JSON.`
	_volumeInspectCommand = &cobra.Command{
		Use:   "inspect [flags] VOLUME [VOLUME...]",
		Short: "Display detailed information on one or more volumes",
		Long:  volumeInspectDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			volumeInspectCommand.InputArgs = args
			volumeInspectCommand.GlobalFlags = MainGlobalOpts
			volumeInspectCommand.Remote = remoteclient
			return volumeInspectCmd(&volumeInspectCommand)
		},
		Example: `podman volume inspect myvol
  podman volume inspect --all
  podman volume inspect --format "{{.Driver}} {{.Scope}}" myvol`,
	}
)

func init() {
	volumeInspectCommand.Command = _volumeInspectCommand
	volumeInspectCommand.SetHelpTemplate(HelpTemplate())
	volumeInspectCommand.SetUsageTemplate(UsageTemplate())
	flags := volumeInspectCommand.Flags()
	flags.BoolVarP(&volumeInspectCommand.All, "all", "a", false, "Inspect all volumes")
	flags.StringVarP(&volumeInspectCommand.Format, "format", "f", "json", "Format volume output using Go template")

}

func volumeInspectCmd(c *cliconfig.VolumeInspectValues) error {
	if (c.All && len(c.InputArgs) > 0) || (!c.All && len(c.InputArgs) < 1) {
		return errors.New("provide one or more volume names or use --all")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	vols, err := runtime.InspectVolumes(getContext(), c)
	if err != nil {
		return err
	}
	return generateVolLsOutput(vols, volumeLsOptions{Format: c.Format})
}
