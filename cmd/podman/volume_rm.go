package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var volumeRmDescription = `
podman volume rm

Remove one or more existing volumes. Will only remove volumes that are
not being used by any containers. To remove the volumes anyways, use the
--force flag.
`

var volumeRmFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "all, a",
		Usage: "Remove all volumes",
	},
	cli.BoolFlag{
		Name:  "force, f",
		Usage: "Remove a volume by force, even if it is being used by a container",
	},
}

var volumeRmCommand = cli.Command{
	Name:                   "rm",
	Aliases:                []string{"remove"},
	Usage:                  "Remove one or more volumes",
	Description:            volumeRmDescription,
	Flags:                  volumeRmFlags,
	Action:                 volumeRmCmd,
	ArgsUsage:              "[VOLUME-NAME ...]",
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
}

func volumeRmCmd(c *cli.Context) error {
	var err error

	if err = validateFlags(c, volumeRmFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	ctx := getContext()

	vols, lastError := getVolumesFromContext(c, runtime)
	for _, vol := range vols {
		err = runtime.RemoveVolume(ctx, vol, c.Bool("force"), false)
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
