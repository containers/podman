package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	installFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "authfile",
			Usage: "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
		cli.BoolFlag{
			Name:  "display",
			Usage: "preview the command that `podman install` would execute",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Usage: "`pathname` of a directory containing TLS certificates and keys",
		},
		cli.StringFlag{
			Name:  "creds",
			Usage: "`credentials` (USERNAME:PASSWORD) to use for authenticating to a registry",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "Assign a name to the container",
		},
		cli.StringFlag{
			Name:  "opt1",
			Usage: "Optional parameter to pass for install",
		},
		cli.StringFlag{
			Name:  "opt2",
			Usage: "Optional parameter to pass for install",
		},
		cli.StringFlag{
			Name:  "opt3",
			Usage: "Optional parameter to pass for install",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress output information when installing images",
		},
		cli.StringFlag{
			Name:  "signature-policy",
			Usage: "`pathname` of signature policy file (not usually used)",
		},
		cli.BoolTFlag{
			Name:  "tls-verify",
			Usage: "require HTTPS and verify certificates when contacting registries (default: true)",
		},
	}

	installDescription = `
Installs an image from a registry and stores it locally.
An image can be installed using its tag or digest. If a tag is not
specified, the image with the 'latest' tag (if it exists) is installed
`
	installCommand = cli.Command{
		Name:         "install",
		Usage:        "Install an image from a registry",
		Description:  installDescription,
		Flags:        installFlags,
		Action:       installCmd,
		ArgsUsage:    "",
		OnUsageError: usageErrorHandler,
	}
)

// installCmd gets the data from the command line and calls installImage
// to copy an image from a registry to a local machine
func installCmd(c *cli.Context) error {
	var (
		imageName      string
		stdErr, stdOut io.Writer
		stdIn          io.Reader
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
	if err := validateFlags(c, installFlags); err != nil {
		return err
	}

	if c.Bool("display") && c.Bool("quiet") {
		return errors.Errorf("the display and quiet flags cannot be used together.")
	}
	if c.IsSet("opts1") {
		opts["opts1"] = c.String("opts1")
	}
	if c.IsSet("opts2") {
		opts["opts2"] = c.String("opts2")
	}
	if c.IsSet("opts3") {
		opts["opts3"] = c.String("opts3")
	}

	ctx := getContext()
	rtc := runtime.GetConfig()

	stdErr = os.Stderr
	stdOut = os.Stdout
	stdIn = os.Stdin

	if c.Bool("quiet") {
		stdErr = nil
		stdOut = nil
		stdIn = nil
	}

	newImage, err := runtime.ImageRuntime().New(ctx, c.Args()[0], rtc.SignaturePolicyPath, "", stdOut, nil, image.SigningOptions{}, false, false)
	if err != nil {
		return errors.Wrapf(err, "unable to find image")
	}

	if len(newImage.Names()) < 1 {
		imageName = newImage.ID()
	} else {
		imageName = newImage.Names()[0]
	}

	installLabel, err := newImage.GetLabel(ctx, "install")
	if err != nil {
		return err
	}

	// If not label to execute, we return
	if installLabel == "" {
		return nil
	}

	cmd := shared.GenerateCommand(installLabel, imageName, c.String("name"))
	env := shared.GenerateRunEnvironment(c.String("name"), imageName, opts)

	if !c.Bool("quiet") {
		fmt.Printf("Running install command: %s\n", strings.Join(cmd, " "))
		if c.Bool("display") {
			return nil
		}
	}
	return utils.ExecCmdWithStdStreams(stdIn, stdOut, stdErr, env, cmd[0], cmd[1:]...)
}
