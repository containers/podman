package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
	"strings"
)

var (
	uninstallFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "display",
			Usage: "preview the command that `podman uninstall` would execute",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "remove containers based on this image",
		},
		cli.StringFlag{
			Name:  "name, -n",
			Usage: "Assign a name to the container",
		},
	}

	uninstallDescription = `
Read the LABEL UNINSTALL field in the container, if it does not exist uninstall will remove the image from your machine.
The UNINSTALL LABEL can look like:

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
	var (
		imageName string
	)

	opts := make(map[string]string)
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

	ctx := getContext()
	newImage, err := runtime.ImageRuntime().NewFromLocal(args[0])
	if err != nil {
		return errors.Wrapf(err, "unable to find image %q in local storage", args[0])
	}

	containers, err := newImage.Containers()
	if err != nil {
		return errors.Wrapf(err, "unable to get a list of containers for the image")
	}

	if len(containers) > 0 && !c.Bool("force") {
		return errors.Errorf("containers based on this image exist. delete them first or pass --force to delete them.")
	}

	if len(newImage.Names()) < 1 {
		imageName = newImage.ID()
	} else {
		imageName = newImage.Names()[0]
	}

	uninstallLabel, err := newImage.GetLabel(ctx, "uninstall")
	if err != nil {
		return err
	}

	// If there is no cmd to execute, we return
	if uninstallLabel == "" {
		return nil
	}
	cmd := shared.GenerateCommand(uninstallLabel, imageName, c.String("name"))
	env := shared.GenerateRunEnvironment(c.String("name"), imageName, opts)

	fmt.Printf("Running uninstall command: %s\n", strings.Join(cmd, " "))
	if c.Bool("display") {
		return nil
	}

	// Run the uninstall command
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, cmd[0], cmd[1:]...); err != nil {
		return err
	}

	// Delete any associated containers
	if _, err := runtime.RemoveImage(ctx, newImage, c.Bool("force")); err != nil {
		return err
	}
	fmt.Println(newImage.ID())
	return nil
}
