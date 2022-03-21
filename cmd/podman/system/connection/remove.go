package connection

import (
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/system"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
		Parent:  system.ConnectionCmd,
	})

	flags := rmCmd.Flags()
	flags.BoolVarP(&rmOpts.All, "all", "a", false, "Remove all connections")
}

func rm(cmd *cobra.Command, args []string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	if rmOpts.All {
		if cfg.Engine.ServiceDestinations != nil {
			for k := range cfg.Engine.ServiceDestinations {
				delete(cfg.Engine.ServiceDestinations, k)
			}
		}
		cfg.Engine.ActiveService = ""
		return cfg.Write()
	}

	if len(args) != 1 {
		return errors.New("accepts 1 arg(s), received 0")
	}

	if cfg.Engine.ServiceDestinations != nil {
		delete(cfg.Engine.ServiceDestinations, args[0])
	}

	if cfg.Engine.ActiveService == args[0] {
		cfg.Engine.ActiveService = ""
	}

	return cfg.Write()
}
