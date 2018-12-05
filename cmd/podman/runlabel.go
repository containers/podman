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
			Usage: "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json. Use REGISTRY_AUTH_FILE environment variable to override. ",
		},
		cli.BoolFlag{
			Name:  "display",
			Usage: "preview the command that the label would run",
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
		Name:           "runlabel",
		Usage:          "Execute the command described by an image label",
		Description:    runlabelDescription,
		Flags:          sortFlags(runlabelFlags),
		Action:         runlabelCmd,
		ArgsUsage:      "",
		SkipArgReorder: true,
		OnUsageError:   usageErrorHandler,
	}
)

// installCmd gets the data from the command line and calls installImage
// to copy an image from a registry to a local machine
func runlabelCmd(c *cli.Context) error {
	var (
		imageName      string
		stdErr, stdOut io.Writer
		stdIn          io.Reader
		extraArgs      []string
	)

	// Evil images could trick into recursively executing the runlabel
	// command.  Avoid this by setting the "PODMAN_RUNLABEL_NESTED" env
	// variable when executing a label first.
	nested := os.Getenv("PODMAN_RUNLABEL_NESTED")
	if nested == "1" {
		return fmt.Errorf("nested runlabel calls: runlabels cannot execute the runlabel command")
	}

	opts := make(map[string]string)
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) < 2 {
		logrus.Errorf("the runlabel command requires at least 2 arguments: LABEL IMAGE")
		return nil
	}
	if err := validateFlags(c, runlabelFlags); err != nil {
		return err
	}
	if c.Bool("display") && c.Bool("quiet") {
		return errors.Errorf("the display and quiet flags cannot be used together.")
	}

	if len(args) > 2 {
		extraArgs = args[2:]
	}
	pull := c.Bool("pull")
	label := args[0]

	runlabelImage := args[1]

	if c.IsSet("opt1") {
		opts["opt1"] = c.String("opt1")
	}
	if c.IsSet("opt2") {
		opts["opt2"] = c.String("opt2")
	}
	if c.IsSet("opt3") {
		opts["opt3"] = c.String("opt3")
	}

	ctx := getContext()

	stdErr = os.Stderr
	stdOut = os.Stdout
	stdIn = os.Stdin

	if c.Bool("quiet") {
		stdErr = nil
		stdOut = nil
		stdIn = nil
	}

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerCertPath:              c.String("cert-dir"),
		DockerInsecureSkipTLSVerify: !c.BoolT("tls-verify"),
	}

	authfile := getAuthFile(c.String("authfile"))
	runLabel, imageName, err := shared.GetRunlabel(label, runlabelImage, ctx, runtime, pull, c.String("creds"), dockerRegistryOptions, authfile, c.String("signature-policy"), stdOut)
	if err != nil {
		return err
	}
	if runLabel == "" {
		return nil
	}

	cmd, env, err := shared.GenerateRunlabelCommand(runLabel, imageName, c.String("name"), opts, extraArgs)
	if err != nil {
		return err
	}
	if !c.Bool("quiet") {
		fmt.Printf("Command: %s\n", strings.Join(cmd, " "))
		if c.Bool("display") {
			return nil
		}
	}
	return utils.ExecCmdWithStdStreams(stdIn, stdOut, stdErr, env, cmd[0], cmd[1:]...)
}
