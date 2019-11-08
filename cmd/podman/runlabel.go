package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
			runlabelCommand.Remote = remoteclient
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
	flags.StringVar(&runlabelCommand.Creds, "creds", "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	flags.BoolVar(&runlabelCommand.Display, "display", false, "Preview the command that the label would run")
	flags.BoolVar(&runlabelCommand.Replace, "replace", false, "Replace existing container with a new one from the image")
	flags.StringVar(&runlabelCommand.Name, "name", "", "Assign a name to the container")

	flags.StringVar(&runlabelCommand.Opt1, "opt1", "", "Optional parameter to pass for install")
	flags.StringVar(&runlabelCommand.Opt2, "opt2", "", "Optional parameter to pass for install")
	flags.StringVar(&runlabelCommand.Opt3, "opt3", "", "Optional parameter to pass for install")
	markFlagHidden(flags, "opt1")
	markFlagHidden(flags, "opt2")
	markFlagHidden(flags, "opt3")
	flags.BoolP("pull", "p", false, "Pull the image if it does not exist locally prior to executing the label contents")
	flags.BoolVarP(&runlabelCommand.Quiet, "quiet", "q", false, "Suppress output information when installing images")
	// Disabled flags for the remote client
	if !remote {
		flags.StringVar(&runlabelCommand.Authfile, "authfile", buildahcli.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
		flags.StringVar(&runlabelCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
		flags.StringVar(&runlabelCommand.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
		flags.BoolVar(&runlabelCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

		if err := flags.MarkDeprecated("pull", "podman will pull if not found in local storage"); err != nil {
			logrus.Error("unable to mark pull flag deprecated")
		}
		markFlagHidden(flags, "signature-policy")
	}
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
	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	if c.Authfile != "" {
		if _, err := os.Stat(c.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", c.Authfile)
		}
	}

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

	runLabel, imageName, err := shared.GetRunlabel(label, runlabelImage, ctx, runtime, true, c.Creds, dockerRegistryOptions, c.Authfile, c.SignaturePolicy, stdOut)
	if err != nil {
		return err
	}
	if runLabel == "" {
		return errors.Errorf("%s does not have a label of %s", runlabelImage, label)
	}

	globalOpts := util.GetGlobalOpts(c)
	cmd, env, err := shared.GenerateRunlabelCommand(runLabel, imageName, c.Name, opts, extraArgs, globalOpts)
	if err != nil {
		return err
	}
	if !c.Quiet {
		fmt.Printf("command: %s\n", strings.Join(append([]string{os.Args[0]}, cmd[1:]...), " "))
		if c.Display {
			return nil
		}
	}

	// If container already exists && --replace given -- Nuke it
	if c.Replace {
		for i, entry := range cmd {
			if entry == "--name" {
				name := cmd[i+1]
				ctr, err := runtime.LookupContainer(name)
				if err != nil {
					if errors.Cause(err) != define.ErrNoSuchCtr {
						logrus.Debugf("Error occurred searching for container %s: %s", name, err.Error())
						return err
					}
				} else {
					logrus.Debugf("Runlabel --replace option given. Container %s will be deleted. The new container will be named %s", ctr.ID(), name)
					if err := runtime.RemoveContainer(ctx, ctr, true, false); err != nil {
						return err
					}
				}
				break
			}
		}
	}

	return utils.ExecCmdWithStdStreams(stdIn, stdOut, stdErr, env, cmd[0], cmd[1:]...)
}
