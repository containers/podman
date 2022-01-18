package connection

import (
	"fmt"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/system"
	"github.com/spf13/cobra"
)

var (
	// Skip creating engines since this command will obtain connection information to said engines
	renameCmd = &cobra.Command{
		Use:               "rename OLD NEW",
		Aliases:           []string{"mv"},
		Args:              cobra.ExactArgs(2),
		Short:             "Rename \"old\" to \"new\"",
		Long:              `Rename destination for the Podman service from "old" to "new"`,
		ValidArgsFunction: common.AutocompleteSystemConnections,
		RunE:              rename,
		Example: `podman system connection rename laptop devl,
  podman system connection mv laptop devl`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: renameCmd,
		Parent:  system.ConnectionCmd,
	})
}

func rename(cmd *cobra.Command, args []string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	if _, found := cfg.Engine.ServiceDestinations[args[0]]; !found {
		return fmt.Errorf("%q destination is not defined. See \"podman system connection add ...\" to create a connection", args[0])
	}

	cfg.Engine.ServiceDestinations[args[1]] = cfg.Engine.ServiceDestinations[args[0]]
	delete(cfg.Engine.ServiceDestinations, args[0])

	if cfg.Engine.ActiveService == args[0] {
		cfg.Engine.ActiveService = args[1]
	}

	return cfg.Write()
}
