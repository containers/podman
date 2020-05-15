package containers

import (
	"os"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	attachDescription = "The podman attach command allows you to attach to a running container using the container's ID or name, either to view its ongoing output or to control it interactively."
	attachCommand     = &cobra.Command{
		Use:   "attach [flags] CONTAINER",
		Short: "Attach to a running container",
		Long:  attachDescription,
		RunE:  attach,
		Args:  validate.IdOrLatestArgs,
		Example: `podman attach ctrID
  podman attach 1234
  podman attach --no-stdin foobar`,
	}

	containerAttachCommand = &cobra.Command{
		Use:   attachCommand.Use,
		Short: attachCommand.Short,
		Long:  attachCommand.Long,
		RunE:  attachCommand.RunE,
		Args:  validate.IdOrLatestArgs,
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
	flags.BoolVarP(&attachOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: attachCommand,
	})
	flags := attachCommand.Flags()
	attachFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerAttachCommand,
		Parent:  containerCmd,
	})
	containerAttachFlags := containerAttachCommand.Flags()
	attachFlags(containerAttachFlags)
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
