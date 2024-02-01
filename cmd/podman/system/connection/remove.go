package connection

import (
	"errors"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/system"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var (
	// Skip creating engines since this command will obtain connection information to said engines.
	rmCmd = &cobra.Command{
		Use:               "remove [options] NAME",
		Aliases:           []string{"rm"},
		Long:              `Delete named destination from podman configuration`,
		Short:             "Delete named destination",
		ValidArgsFunction: common.AutocompleteSystemConnections,
		RunE:              rm,
		Example: `podman system connection remove devl
  podman system connection rm devl`,
	}

	rmOpts = struct {
		All bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  system.ContextCmd,
	})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  system.ConnectionCmd,
	})

	flags := rmCmd.Flags()
	flags.BoolVarP(&rmOpts.All, "all", "a", false, "Remove all connections")

	flags.BoolP("force", "f", false, "Ignored: for Docker compatibility")
	_ = flags.MarkHidden("force")
}

func rm(cmd *cobra.Command, args []string) error {
	return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		if rmOpts.All {
			cfg.Connection.Connections = nil
			cfg.Connection.Default = ""

			// Clear all the connections in any existing farms
			for k := range cfg.Farm.List {
				cfg.Farm.List[k] = []string{}
			}
			return nil
		}

		if len(args) != 1 {
			return errors.New("accepts 1 arg(s), received 0")
		}

		delete(cfg.Connection.Connections, args[0])
		if cfg.Connection.Default == args[0] {
			cfg.Connection.Default = ""
		}

		// If there are existing farm, remove the deleted connection that might be part of a farm
		for k, v := range cfg.Farm.List {
			index := slices.Index(v, args[0])
			if index > -1 {
				cfg.Farm.List[k] = append(v[:index], v[index+1:]...)
			}
		}

		return nil
	})
}
