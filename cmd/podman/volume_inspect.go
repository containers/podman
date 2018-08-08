package main

import (
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var volumeInspectDescription = `
podman volume inspect

Display detailed information on one or more volumes. Can change the format
from JSON to a Go template.
`

var volumeInspectFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "all, a",
		Usage: "Inspect all volumes",
	},
	cli.StringFlag{
		Name:  "format, f",
		Usage: "Format volume output using Go template",
		Value: "json",
	},
}

var volumeInspectCommand = cli.Command{
	Name:                   "inspect",
	Usage:                  "Display detailed information on one or more volumes",
	Description:            volumeInspectDescription,
	Flags:                  volumeInspectFlags,
	Action:                 volumeInspectCmd,
	SkipArgReorder:         true,
	ArgsUsage:              "[VOLUME-NAME ...]",
	UseShortOptionHandling: true,
}

func volumeInspectCmd(c *cli.Context) error {
	var err error

	if err = validateFlags(c, volumeInspectFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	opts := volumeLsOptions{
		Format: c.String("format"),
	}

	vols, lastError := getVolumesFromContext(c, runtime)
	if lastError != nil {
		logrus.Errorf("%q", lastError)
	}

	return generateVolLsOutput(vols, opts, runtime)
}
