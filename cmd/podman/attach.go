package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	attachCommand     cliconfig.AttachValues
	attachDescription = "The podman attach command allows you to attach to a running container using the container's ID or name, either to view its ongoing output or to control it interactively."
	_attachCommand    = &cobra.Command{
		Use:   "attach [flags] CONTAINER",
		Short: "Attach to a running container",
		Long:  attachDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			attachCommand.InputArgs = args
			attachCommand.GlobalFlags = MainGlobalOpts
			attachCommand.Remote = remoteclient
			return attachCmd(&attachCommand)
		},
		Example: `podman attach ctrID
  podman attach 1234
  podman attach --no-stdin foobar`,
	}
)

func init() {
	attachCommand.Command = _attachCommand
	attachCommand.SetHelpTemplate(HelpTemplate())
	attachCommand.SetUsageTemplate(UsageTemplate())
	flags := attachCommand.Flags()
	flags.StringVar(&attachCommand.DetachKeys, "detach-keys", define.DefaultDetachKeys, "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	flags.BoolVar(&attachCommand.NoStdin, "no-stdin", false, "Do not attach STDIN. The default is false")
	flags.BoolVar(&attachCommand.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
	flags.BoolVarP(&attachCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
	// TODO allow for passing of a new deatch keys
	markFlagHiddenForRemoteClient("detach-keys", flags)
}

func attachCmd(c *cliconfig.AttachValues) error {
	if len(c.InputArgs) > 1 || (len(c.InputArgs) == 0 && !c.Latest) {
		return errors.Errorf("attach requires the name or id of one running container or the latest flag")
	}
	if remoteclient && len(c.InputArgs) != 1 {
		return errors.Errorf("attach requires the name or id of one running container")
	}
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating runtime")
	}
	defer runtime.DeferredShutdown(false)
	return runtime.Attach(getContext(), c)
}
