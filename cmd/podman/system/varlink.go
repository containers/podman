// +build linux

package system

import (
	"time"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	varlinkDescription = `Run varlink interface.  Podman varlink listens on the specified unix domain socket for incoming connects.

  Tools speaking varlink protocol can remotely manage pods, containers and images.
`
	varlinkCmd = &cobra.Command{
		Use:   "varlink [flags] [URI]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Run varlink interface",
		Long:  varlinkDescription,
		RunE:  varlinkE,
		Example: `podman varlink unix:/run/podman/io.podman
  podman varlink --time 5000 unix:/run/podman/io.podman`,
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
	flags := varlinkCmd.Flags()
	flags.Int64VarP(&varlinkArgs.Timeout, "time", "t", 1000, "Time until the varlink session expires in milliseconds.  Use 0 to disable the timeout")
	flags.SetNormalizeFunc(aliasTimeoutFlag)
}

func aliasTimeoutFlag(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	if name == "timeout" {
		name = "time"
	}
	return pflag.NormalizedName(name)
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
