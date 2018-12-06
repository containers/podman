package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var volumeCreateDescription = `
podman volume create

Creates a new volume. If using the default driver, "local", the volume will
be created at.`

var volumeCreateFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "driver",
		Usage: "Specify volume driver name (default local)",
	},
	cli.StringSliceFlag{
		Name:  "label, l",
		Usage: "Set metadata for a volume (default [])",
	},
	cli.StringSliceFlag{
		Name:  "opt, o",
		Usage: "Set driver specific options (default [])",
	},
}

var volumeCreateCommand = cli.Command{
	Name:                   "create",
	Usage:                  "Create a new volume",
	Description:            volumeCreateDescription,
	Flags:                  volumeCreateFlags,
	Action:                 volumeCreateCmd,
	SkipArgReorder:         true,
	ArgsUsage:              "[VOLUME-NAME]",
	UseShortOptionHandling: true,
}

func volumeCreateCmd(c *cli.Context) error {
	var (
		options []libpod.VolumeCreateOption
		err     error
		volName string
	)

	if err = validateFlags(c, volumeCreateFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if len(c.Args()) > 1 {
		return errors.Errorf("too many arguments, create takes at most 1 argument")
	}

	if len(c.Args()) > 0 {
		volName = c.Args()[0]
		options = append(options, libpod.WithVolumeName(volName))
	}

	if c.IsSet("driver") {
		options = append(options, libpod.WithVolumeDriver(c.String("driver")))
	}

	labels, err := getAllLabels([]string{}, c.StringSlice("label"))
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}
	if len(labels) != 0 {
		options = append(options, libpod.WithVolumeLabels(labels))
	}

	opts, err := getAllLabels([]string{}, c.StringSlice("opt"))
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
