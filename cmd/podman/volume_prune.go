package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var volumePruneDescription = `
podman volume prune

Remove all unused volumes. Will prompt for confirmation if not
using force.
`

var volumePruneFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "force, f",
		Usage: "Do not prompt for confirmation",
	},
}

var volumePruneCommand = cli.Command{
	Name:                   "prune",
	Usage:                  "Remove all unused volumes",
	Description:            volumePruneDescription,
	Flags:                  volumePruneFlags,
	Action:                 volumePruneCmd,
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
}

func volumePruneCmd(c *cli.Context) error {
	var lastError error

	if err := validateFlags(c, volumePruneFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	ctx := getContext()

	// Prompt for confirmation if --force is not set
	if !c.Bool("force") {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all volumes not used by at least one container.")
		fmt.Print("Are you sure you want to continue? [y/N] ")
		ans, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(ans)[0] != 'y' {
			return nil
		}
	}

	volumes, err := runtime.GetAllVolumes()
	if err != nil {
		return err
	}

	for _, vol := range volumes {
		err = runtime.RemoveVolume(ctx, vol, false, true)
		if err == nil {
			fmt.Println(vol.Name())
		} else if err != libpod.ErrVolumeBeingUsed {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "failed to remove volume %q", vol.Name())
		}
	}
	return lastError
}
