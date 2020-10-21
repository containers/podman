package containers

import (
	"os"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	attachDescription = "The podman attach command allows you to attach to a running container using the container's ID or name, either to view its ongoing output or to control it interactively."
	attachCommand     = &cobra.Command{
		Use:   "attach [options] CONTAINER",
		Short: "Attach to a running container",
		Long:  attachDescription,
		RunE:  attach,
		Args:  validate.IDOrLatestArgs,
		Example: `podman attach ctrID
  podman attach 1234
  podman attach --no-stdin foobar`,
	}

	containerAttachCommand = &cobra.Command{
		Use:   attachCommand.Use,
		Short: attachCommand.Short,
		Long:  attachCommand.Long,
		RunE:  attachCommand.RunE,
		Args:  validate.IDOrLatestArgs,
		Example: `podman container attach ctrID
	podman container attach 1234
	podman container attach --no-stdin foobar`,
	}
)

var (
	attachOpts entities.AttachOptions
)

func attachFlags(flags *pflag.FlagSet) {
	flags.StringVar(&attachOpts.DetachKeys, "detach-keys", containerConfig.DetachKeys(), "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	flags.BoolVar(&attachOpts.NoStdin, "no-stdin", false, "Do not attach STDIN. The default is false")
	flags.BoolVar(&attachOpts.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: attachCommand,
	})
	attachFlags(attachCommand.Flags())
	validate.AddLatestFlag(attachCommand, &attachOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerAttachCommand,
		Parent:  containerCmd,
	})
	attachFlags(containerAttachCommand.Flags())
	validate.AddLatestFlag(containerAttachCommand, &attachOpts.Latest)

}

func attach(cmd *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 0 && !attachOpts.Latest) {
		return errors.Errorf("attach requires the name or id of one running container or the latest flag")
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	}
	attachOpts.Stdin = os.Stdin
	if attachOpts.NoStdin {
		attachOpts.Stdin = nil
	}
	attachOpts.Stdout = os.Stdout
	attachOpts.Stderr = os.Stderr
	return registry.ContainerEngine().ContainerAttach(registry.GetContext(), name, attachOpts)
}
