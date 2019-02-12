package main

import (
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	attachCommand     cliconfig.AttachValues
	attachDescription = "The podman attach command allows you to attach to a running container using the container's ID or name, either to view its ongoing output or to control it interactively."
	_attachCommand    = &cobra.Command{
		Use:   "attach",
		Short: "Attach to a running container",
		Long:  attachDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			attachCommand.InputArgs = args
			attachCommand.GlobalFlags = MainGlobalOpts
			return attachCmd(&attachCommand)
		},
		Example: "",
	}
)

func init() {
	attachCommand.Command = _attachCommand
	attachCommand.SetUsageTemplate(UsageTemplate())
	flags := attachCommand.Flags()
	flags.StringVar(&attachCommand.DetachKeys, "detach-keys", "", "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _")
	flags.BoolVar(&attachCommand.NoStdin, "no-stdin", false, "Do not attach STDIN. The default is false")
	flags.BoolVar(&attachCommand.SigProxy, "sig-proxy", true, "Proxy received signals to the process (default true)")

	flags.BoolVarP(&attachCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
}

func attachCmd(c *cliconfig.AttachValues) error {
	args := c.InputArgs
	var ctr *libpod.Container

	if len(c.InputArgs) > 1 || (len(c.InputArgs) == 0 && !c.Latest) {
		return errors.Errorf("attach requires the name or id of one running container or the latest flag")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if c.Latest {
		ctr, err = runtime.GetLatestContainer()
	} else {
		ctr, err = runtime.LookupContainer(args[0])
	}

	if err != nil {
		return errors.Wrapf(err, "unable to exec into %s", args[0])
	}

	conState, err := ctr.State()
	if err != nil {
		return errors.Wrapf(err, "unable to determine state of %s", args[0])
	}
	if conState != libpod.ContainerStateRunning {
		return errors.Errorf("you can only attach to running containers")
	}

	inputStream := os.Stdin
	if c.NoStdin {
		inputStream = nil
	}

	if err := startAttachCtr(ctr, os.Stdout, os.Stderr, inputStream, c.DetachKeys, c.SigProxy, false); err != nil && errors.Cause(err) != libpod.ErrDetach {
		return errors.Wrapf(err, "error attaching to container %s", ctr.ID())
	}

	return nil
}
