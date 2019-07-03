package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	runCommand cliconfig.RunValues

	runDescription = "Runs a command in a new container from the given image"
	_runCommand    = &cobra.Command{
		Use:   "run [flags] IMAGE [COMMAND [ARG...]]",
		Short: "Run a command in a new container",
		Long:  runDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			runCommand.InputArgs = args
			runCommand.GlobalFlags = MainGlobalOpts
			runCommand.Remote = remoteclient
			return runCmd(&runCommand)
		},
		Example: `podman run imageID ls -alF /etc
  podman run --net=host imageID dnf -y install java
  podman run --volume /var/hostdir:/var/ctrdir -i -t fedora /bin/bash`,
	}
)

func init() {
	runCommand.Command = _runCommand
	runCommand.SetHelpTemplate(HelpTemplate())
	runCommand.SetUsageTemplate(UsageTemplate())
	flags := runCommand.Flags()
	flags.SetInterspersed(false)
	flags.Bool("sig-proxy", true, "Proxy received signals to the process")
	getCreateFlags(&runCommand.PodmanCommand)
	markFlagHiddenForRemoteClient("authfile", flags)
	flags.MarkHidden("signature-policy")
}

func runCmd(c *cliconfig.RunValues) error {
	if !remote && c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "runCmd")
		defer span.Finish()
	}

	if err := createInit(&c.PodmanCommand); err != nil {
		return err
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	exitCode, err = runtime.Run(getContext(), c, exitCode)
	return err
}
