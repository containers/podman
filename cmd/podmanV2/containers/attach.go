package containers

import (
	"os"

	"github.com/containers/libpod/cmd/podmanV2/common"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	attachDescription = "The podman attach command allows you to attach to a running container using the container's ID or name, either to view its ongoing output or to control it interactively."
	attachCommand     = &cobra.Command{
		Use:   "attach [flags] CONTAINER",
		Short: "Attach to a running container",
		Long:  attachDescription,
		RunE:  attach,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 || (len(args) == 0 && !cmd.Flag("latest").Changed) {
				return errors.Errorf("attach requires the name or id of one running container or the latest flag")
			}
			return nil
		},
		PreRunE: preRunE,
		Example: `podman attach ctrID
  podman attach 1234
  podman attach --no-stdin foobar`,
	}
)

var (
	attachOpts entities.AttachOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: attachCommand,
	})
	flags := attachCommand.Flags()
	flags.StringVar(&attachOpts.DetachKeys, "detach-keys", common.GetDefaultDetachKeys(), "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	flags.BoolVar(&attachOpts.NoStdin, "no-stdin", false, "Do not attach STDIN. The default is false")
	flags.BoolVar(&attachOpts.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
	flags.BoolVarP(&attachOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func attach(cmd *cobra.Command, args []string) error {
	attachOpts.Stdin = os.Stdin
	if attachOpts.NoStdin {
		attachOpts.Stdin = nil
	}
	attachOpts.Stdout = os.Stdout
	attachOpts.Stderr = os.Stderr
	return registry.ContainerEngine().ContainerAttach(registry.GetContext(), args[0], attachOpts)
}
