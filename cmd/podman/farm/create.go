package farm

import (
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var (
	farmCreateDescription = `Create a new farm with connections added via podman system connection add.

	The "podman system connection add --farm" command can be used to add a new connection to a new or existing farm.`

	createCommand = &cobra.Command{
		Use:               "create [options] NAME [CONNECTIONS...]",
		Args:              cobra.MinimumNArgs(1),
		Short:             "Create a new farm",
		Long:              farmCreateDescription,
		RunE:              create,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman farm create myfarm connection1
  podman farm create myfarm`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCommand,
		Parent:  farmCmd,
	})
}

func create(cmd *cobra.Command, args []string) error {
	farmName := args[0]
	connections := args[1:]

	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}

	if _, ok := cfg.Farms.List[farmName]; ok {
		// if farm exists return an error
		return fmt.Errorf("farm with name %q already exists", farmName)
	}

	// Can create an empty farm without any connections
	if len(connections) == 0 {
		cfg.Farms.List[farmName] = []string{}
	}

	for _, c := range connections {
		if _, ok := cfg.Engine.ServiceDestinations[c]; ok {
			if slices.Contains(cfg.Farms.List[farmName], c) {
				// Don't add duplicate connections to a farm
				continue
			}
			cfg.Farms.List[farmName] = append(cfg.Farms.List[farmName], c)
		} else {
			return fmt.Errorf("cannot create farm, %q is not a system connection", c)
		}
	}

	// If this is the first farm being created, set it as the default farm
	if len(cfg.Farms.List) == 1 {
		cfg.Farms.Default = farmName
	}

	err = cfg.Write()
	if err != nil {
		return err
	}

	fmt.Printf("Farm %q created\n", farmName)
	return nil
}
