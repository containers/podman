package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	uninstallFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "display",
			Usage: "preview the command that `podman uninstall` would execute",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "Assign a name to the container",
		},
	}

	uninstallDescription = `
Read the LABEL UNINSTALL field in the container, if it does not exist uninstall will remove the image from your machine. 
THe UNINSTALL LABEL can look like: 

LABEL UNINSTALL podman run -t -i --rm --privileged -v /:/host --net=host --ipc=host --pid=host -e HOST=/host -e NAME=${NAME} -e IMAGE=${IMAGE} -e CONFDIR=/host/etc/${NAME} -e LOGDIR=/host/var/log/${NAME} -e DATADIR=/host/var/lib/${NAME} -e SYSTEMD_IGNORE_CHROOT=1 --name ${NAME} ${IMAGE} /usr/bin/UNINSTALLCMD
`
	uninstallCommand = cli.Command{
		Name:         "uninstall",
		Usage:        "Execute container image uninstall method",
		Description:  uninstallDescription,
		Flags:        uninstallFlags,
		Action:       uninstallCmd,
		ArgsUsage:    "",
		OnUsageError: usageErrorHandler,
	}
)

// uninstallCmd gets the data from the command line and calls uninstallImage
// to copy an image from a registry to a local machine
func uninstallCmd(c *cli.Context) error {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) == 0 {
		logrus.Errorf("an image name must be specified")
		return nil
	}
	if len(args) > 1 {
		logrus.Errorf("too many arguments. Requires exactly 1")
		return nil
	}
	if err := validateFlags(c, uninstallFlags); err != nil {
		return err
	}
	image := args[0]

	newImage, err := getImage(c, runtime, image)
	if err != nil {
		return errors.Wrapf(err, "error uninstalling image %q", image)
	}

	data, err := newImage.Inspect(getContext())
	if err != nil {
		return errors.Wrapf(err, "error parsing image data %q", newImage.ID())
	}
	fmt.Println(data.Labels["UNINSTALL"])
	return nil
}
