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
	runlabelFlags = []cli.Flag{
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
			Name:   "opt1",
			Usage:  "Optional parameter to pass for install",
			Hidden: true,
		},
		cli.StringFlag{
			Name:   "opt2",
			Usage:  "Optional parameter to pass for install",
			Hidden: true,
		},
		cli.StringFlag{
			Name:   "opt3",
			Usage:  "Optional parameter to pass for install",
			Hidden: true,
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress output information when installing images",
		},
		cli.BoolFlag{
			Name:  "pull, p",
			Usage: "pull the image if it does not exist locally prior to executing the label contents",
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

	runlabelDescription = `
Executes a command as described by a container image label.
`
	runlabelCommand = cli.Command{
		Name:         "runlabel",
		Usage:        "Execute the command described by an image label",
		Description:  runlabelDescription,
		Flags:        runlabelFlags,
		Action:       runlabelCmd,
		ArgsUsage:    "",
		OnUsageError: usageErrorHandler,
	}
)

// installCmd gets the data from the command line and calls installImage
// to copy an image from a registry to a local machine
func runlabelCmd(c *cli.Context) error {
	var (
		imageName      string
		stdErr, stdOut io.Writer
		stdIn          io.Reader
		newImage       *image.Image
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
	if len(args) < 2 {
		logrus.Errorf("the runlabel command requires at least 2 arguments")
		return nil
	}
	if err := validateFlags(c, runlabelFlags); err != nil {
		return err
	}
	if c.Bool("display") && c.Bool("quiet") {
		return errors.Errorf("the display and quiet flags cannot be used together.")
	}

	pull := c.Bool("pull")
	label := args[0]

	runlabelImage := args[1]

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

	if pull {
		newImage, err = runtime.ImageRuntime().New(ctx, runlabelImage, rtc.SignaturePolicyPath, "", stdOut, nil, image.SigningOptions{}, false, false)
	} else {
		newImage, err = runtime.ImageRuntime().NewFromLocal(runlabelImage)
	}
	if err != nil {
		return errors.Wrapf(err, "unable to find image")
	}

	if len(newImage.Names()) < 1 {
		imageName = newImage.ID()
	} else {
		imageName = newImage.Names()[0]
	}

	runLabel, err := newImage.GetLabel(ctx, label)
	if err != nil {
		return err
	}

	// If no label to execute, we return
	if runLabel == "" {
		return nil
	}

	// The user provided extra arguments that need to be tacked onto the label's command
	if len(args) > 2 {
		runLabel = fmt.Sprintf("%s %s", runLabel, strings.Join(args[2:], " "))
	}

	cmd := shared.GenerateCommand(runLabel, imageName, c.String("name"))
	env := shared.GenerateRunEnvironment(c.String("name"), imageName, opts)

	if !c.Bool("quiet") {
		fmt.Printf("Command: %s\n", strings.Join(cmd, " "))
		if c.Bool("display") {
			return nil
		}
	}
	return utils.ExecCmdWithStdStreams(stdIn, stdOut, stdErr, env, cmd[0], cmd[1:]...)
}
