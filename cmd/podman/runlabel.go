package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	runlabelCommand     cliconfig.RunlabelValues
	runlabelDescription = `
Executes a command as described by a container image label.
`
	_runlabelCommand = &cobra.Command{
		Use:   "runlabel [flags] LABEL IMAGE [ARG...]",
		Short: "Execute the command described by an image label",
		Long:  runlabelDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			runlabelCommand.InputArgs = args
			runlabelCommand.GlobalFlags = MainGlobalOpts
			return runlabelCmd(&runlabelCommand)
		},
		Example: `podman container runlabel run imageID
  podman container runlabel --pull install imageID arg1 arg2
  podman container runlabel --display run myImage`,
	}
)

func init() {
	runlabelCommand.Command = _runlabelCommand
	runlabelCommand.SetHelpTemplate(HelpTemplate())
	runlabelCommand.SetUsageTemplate(UsageTemplate())
	flags := runlabelCommand.Flags()
	flags.StringVar(&runlabelCommand.Authfile, "authfile", "", "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json. Use REGISTRY_AUTH_FILE environment variable to override")
	flags.StringVar(&runlabelCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
	flags.StringVar(&runlabelCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVar(&runlabelCommand.Display, "display", false, "Preview the command that the label would run")
	flags.StringVar(&runlabelCommand.Name, "name", "", "Assign a name to the container")

	flags.StringVar(&runlabelCommand.Opt1, "opt1", "", "Optional parameter to pass for install")
	flags.StringVar(&runlabelCommand.Opt2, "opt2", "", "Optional parameter to pass for install")
	flags.StringVar(&runlabelCommand.Opt3, "opt3", "", "Optional parameter to pass for install")
	flags.MarkHidden("opt1")
	flags.MarkHidden("opt2")
	flags.MarkHidden("opt3")

	flags.BoolP("pull", "p", false, "Pull the image if it does not exist locally prior to executing the label contents")
	flags.BoolVarP(&runlabelCommand.Quiet, "quiet", "q", false, "Suppress output information when installing images")
	flags.BoolVar(&runlabelCommand.Replace, "replace", false, "remove a container with a similar name before executing the label")
	flags.StringVar(&runlabelCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
	flags.BoolVar(&runlabelCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries (default: true)")

	flags.MarkDeprecated("pull", "podman will pull if not found in local storage")
}

// installCmd gets the data from the command line and calls installImage
// to copy an image from a registry to a local machine
func runlabelCmd(c *cliconfig.RunlabelValues) error {
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
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.InputArgs
	if len(args) < 2 {
		return errors.Errorf("the runlabel command requires at least 2 arguments: LABEL IMAGE")
	}
	if c.Display && c.Quiet {
		return errors.Errorf("the display and quiet flags cannot be used together.")
	}

	if len(args) > 2 {
		extraArgs = args[2:]
	}
	label := args[0]

	runlabelImage := args[1]

	if c.Flag("opt1").Changed {
		opts["opt1"] = c.Opt1
	}

	if c.Flag("opt2").Changed {
		opts["opt2"] = c.Opt2
	}
	if c.Flag("opt3").Changed {
		opts["opt3"] = c.Opt3
	}

	ctx := getContext()

	stdErr = os.Stderr
	stdOut = os.Stdout
	stdIn = os.Stdin

	if c.Quiet {
		stdErr = nil
		stdOut = nil
		stdIn = nil
	}

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerCertPath: c.CertDir,
	}
	if c.Flag("tls-verify").Changed {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}

	authfile := getAuthFile(c.Authfile)
	runLabel, imageName, err := shared.GetRunlabel(label, runlabelImage, ctx, runtime, true, c.Creds, dockerRegistryOptions, authfile, c.SignaturePolicy, stdOut)
	if err != nil {
		return err
	}
	if runLabel == "" {
		return errors.Errorf("%s does not have a label of %s", runlabelImage, label)
	}

	cmd, env, err := shared.GenerateRunlabelCommand(runLabel, imageName, c.Name, opts, extraArgs)
	if err != nil {
		return err
	}
	if !c.Quiet {
		fmt.Printf("Command: %s\n", strings.Join(cmd, " "))
		if c.Display {
			return nil
		}
	}

	if len(c.Name) > 0 && c.Replace {
		// we need to check if a container with the same name already exists
		ctr, err := runtime.LookupContainer(c.Name)
		if err == nil {
			// a container with the same name exists, remove it
			if err := runtime.RemoveContainer(ctx, ctr, true, true); err != nil {
				return err
			}
		}
	}
	return utils.ExecCmdWithStdStreams(stdIn, stdOut, stdErr, env, cmd[0], cmd[1:]...)
}
