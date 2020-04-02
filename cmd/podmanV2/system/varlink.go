package system

import (
	"time"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	varlinkDescription = `Run varlink interface.  Podman varlink listens on the specified unix domain socket for incoming connects.

  Tools speaking varlink protocol can remotely manage pods, containers and images.
`
	varlinkCmd = &cobra.Command{
		Use:     "varlink [flags] [URI]",
		Args:    cobra.MinimumNArgs(1),
		Short:   "Run varlink interface",
		Long:    varlinkDescription,
		PreRunE: preRunE,
		RunE:    varlinkE,
		Example: `podman varlink unix:/run/podman/io.podman
  podman varlink --timeout 5000 unix:/run/podman/io.podman`,
	}
	varlinkArgs = struct {
		Timeout int64
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: varlinkCmd,
	})
	varlinkCmd.SetHelpTemplate(registry.HelpTemplate())
	varlinkCmd.SetUsageTemplate(registry.UsageTemplate())

	flags := varlinkCmd.Flags()
	flags.Int64VarP(&varlinkArgs.Timeout, "time", "t", 1000, "Time until the varlink session expires in milliseconds.  Use 0 to disable the timeout")
	flags.Int64Var(&varlinkArgs.Timeout, "timeout", 1000, "Time until the varlink session expires in milliseconds.  Use 0 to disable the timeout")

}

func varlinkE(cmd *cobra.Command, args []string) error {
	uri := registry.DefaultVarlinkAddress
	if len(args) > 0 {
		uri = args[0]
	}
	opts := entities.ServiceOptions{
		URI:     uri,
		Timeout: time.Duration(varlinkArgs.Timeout) * time.Second,
		Command: cmd,
	}
	return registry.ContainerEngine().VarlinkService(registry.GetContext(), opts)
}
