package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	execCommand cliconfig.ExecValues

	execDescription = `Execute the specified command inside a running container.
`
	_execCommand = &cobra.Command{
		Use:   "exec [flags] CONTAINER [COMMAND [ARG...]]",
		Short: "Run a process in a running container",
		Long:  execDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			execCommand.InputArgs = args
			execCommand.GlobalFlags = MainGlobalOpts
			execCommand.Remote = remoteclient
			return execCmd(&execCommand)
		},
		Example: `podman exec -it ctrID ls
  podman exec -it -w /tmp myCtr pwd
  podman exec --user root ctrID ls`,
	}
)

func init() {
	execCommand.Command = _execCommand
	execCommand.SetHelpTemplate(HelpTemplate())
	execCommand.SetUsageTemplate(UsageTemplate())
	flags := execCommand.Flags()
	flags.SetInterspersed(false)
	flags.StringVar(&execCommand.DetachKeys, "detach-keys", define.DefaultDetachKeys, "Select the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _")
	flags.StringArrayVarP(&execCommand.Env, "env", "e", []string{}, "Set environment variables")
	flags.BoolVarP(&execCommand.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&execCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&execCommand.Privileged, "privileged", false, "Give the process extended Linux capabilities inside the container.  The default is false")
	flags.BoolVarP(&execCommand.Tty, "tty", "t", false, "Allocate a pseudo-TTY. The default is false")
	flags.StringVarP(&execCommand.User, "user", "u", "", "Sets the username or UID used and optionally the groupname or GID for the specified command")

	flags.IntVar(&execCommand.PreserveFDs, "preserve-fds", 0, "Pass N additional file descriptors to the container")
	flags.StringVarP(&execCommand.Workdir, "workdir", "w", "", "Working directory inside the container")
	markFlagHiddenForRemoteClient("latest", flags)
	markFlagHiddenForRemoteClient("preserve-fds", flags)
}

func execCmd(c *cliconfig.ExecValues) error {
	argLen := len(c.InputArgs)
	if c.Latest {
		if argLen < 1 {
			return errors.Errorf("you must provide a command to exec")
		}
	} else {
		if argLen < 1 {
			return errors.Errorf("you must provide one container name or id")
		}
		if argLen < 2 {
			return errors.Errorf("you must provide a command to exec")
		}
	}
	runtime, err := adapter.GetRuntimeNoStore(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	exitCode, err = runtime.ExecContainer(getContext(), c)
	return err
}
