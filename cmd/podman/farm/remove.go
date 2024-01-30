package farm

import (
	"errors"
	"fmt"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	farmRmDescription = `Remove one or more existing farms.`
	rmCommand         = &cobra.Command{
		Use:                "remove [options] [FARM...]",
		Aliases:            []string{"rm"},
		Short:              "Remove one or more farms",
		Long:               farmRmDescription,
		PersistentPreRunE:  validate.NoOp,
		RunE:               rm,
		PersistentPostRunE: validate.NoOp,
		ValidArgsFunction:  common.AutoCompleteFarms,
		Example: `podman farm rm myfarm1 myfarm2
  podman farm rm --all`,
	}

	// Temporary struct to hold cli values.
	rmOpts = struct {
		All bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCommand,
		Parent:  farmCmd,
	})
	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOpts.All, "all", "a", false, "Remove all farms")
}

func rm(cmd *cobra.Command, args []string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	if rmOpts.All {
		cfg.Farms.List = make(map[string][]string)
		cfg.Farms.Default = ""
		if err := cfg.Write(); err != nil {
			return err
		}
		fmt.Println("All farms have been deleted")
		return nil
	}

	// If the --all is not set, we require at least one arg
	if len(args) == 0 {
		return errors.New("requires at lease 1 arg(s), received 0")
	}

	if len(cfg.Farms.List) == 0 {
		return errors.New("no existing farms; nothing to remove")
	}

	deletedFarms := []string{}
	for _, k := range args {
		if _, ok := cfg.Farms.List[k]; !ok {
			logrus.Warnf("farm %q doesn't exist; nothing to remove", k)
			continue
		}
		delete(cfg.Farms.List, k)
		deletedFarms = append(deletedFarms, k)
		if k == cfg.Farms.Default {
			cfg.Farms.Default = ""
		}
	}
	// Return error if none of the given farms were deleted
	if len(deletedFarms) == 0 {
		return fmt.Errorf("failed to delete farms %q", args)
	}

	// Set a new default farm if the current default farm has been removed
	if cfg.Farms.Default == "" && cfg.Farms.List != nil {
		for k := range cfg.Farms.List {
			cfg.Farms.Default = k
			break
		}
	}
	if err := cfg.Write(); err != nil {
		return err
	}

	for _, k := range deletedFarms {
		fmt.Printf("Farm %q deleted\n", k)
	}
	return nil
}
