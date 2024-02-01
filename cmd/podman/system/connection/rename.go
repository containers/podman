package connection

import (
	"fmt"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/system"
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
	return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		if _, found := cfg.Connection.Connections[args[0]]; !found {
			return fmt.Errorf("%q destination is not defined. See \"podman system connection add ...\" to create a connection", args[0])
		}

		cfg.Connection.Connections[args[1]] = cfg.Connection.Connections[args[0]]
		delete(cfg.Connection.Connections, args[0])

		if cfg.Connection.Default == args[0] {
			cfg.Connection.Default = args[1]
		}

		return nil
	})
}
