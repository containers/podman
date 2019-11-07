package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	startCommand     cliconfig.StartValues
	startDescription = `Starts one or more containers.  The container name or ID can be used.`

	_startCommand = &cobra.Command{
		Use:   "start [flags] CONTAINER [CONTAINER...]",
		Short: "Start one or more containers",
		Long:  startDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			startCommand.InputArgs = args
			startCommand.GlobalFlags = MainGlobalOpts
			startCommand.Remote = remoteclient
			return startCmd(&startCommand)
		},
		Example: `podman start --latest
  podman start 860a4b231279 5421ab43b45
  podman start --interactive --attach imageID`,
	}
)

func init() {
	startCommand.Command = _startCommand
	startCommand.SetHelpTemplate(HelpTemplate())
	startCommand.SetUsageTemplate(UsageTemplate())
	flags := startCommand.Flags()
	flags.BoolVarP(&startCommand.Attach, "attach", "a", false, "Attach container's STDOUT and STDERR")
	flags.StringVar(&startCommand.DetachKeys, "detach-keys", define.DefaultDetachKeys, "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	flags.BoolVarP(&startCommand.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&startCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&startCommand.SigProxy, "sig-proxy", false, "Proxy received signals to the process (default true if attaching, false otherwise)")
	markFlagHiddenForRemoteClient("latest", flags)
}

func startCmd(c *cliconfig.StartValues) error {
	if !remoteclient && c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "startCmd")
		defer span.Finish()
	}

	args := c.InputArgs
	if len(args) < 1 && !c.Latest {
		return errors.Errorf("you must provide at least one container name or id")
	}

	attach := c.Attach

	if len(args) > 1 && attach {
		return errors.Errorf("you cannot start and attach multiple containers at once")
	}

	sigProxy := c.SigProxy || attach
	if c.Flag("sig-proxy").Changed {
		sigProxy = c.SigProxy
	}

	if sigProxy && !attach {
		return errors.Wrapf(define.ErrInvalidArg, "you cannot use sig-proxy without --attach")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)
	exitCode, err = runtime.Start(getContext(), c, sigProxy)
	return err
}
