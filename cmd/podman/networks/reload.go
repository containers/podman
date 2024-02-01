package network

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networkReloadDescription = `Reload container networks, recreating firewall rules`
	networkReloadCommand     = &cobra.Command{
		Annotations: map[string]string{registry.EngineMode: registry.ABIMode},
		Use:         "reload [options] [CONTAINER...]",
		Short:       "Reload firewall rules for one or more containers",
		Long:        networkReloadDescription,
		RunE:        networkReload,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "")
		},
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman network reload 3c13ef6dd843
  podman network reload test1 test2`,
	}
)

var (
	reloadOptions entities.NetworkReloadOptions
)

func reloadFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&reloadOptions.All, "all", "a", false, "Reload network configuration of all containers")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkReloadCommand,
		Parent:  networkCmd,
	})
	reloadFlags(networkReloadCommand.Flags())
	validate.AddLatestFlag(networkReloadCommand, &reloadOptions.Latest)
}

func networkReload(cmd *cobra.Command, args []string) error {
	responses, err := registry.ContainerEngine().NetworkReload(registry.Context(), args, reloadOptions)
	if err != nil {
		return err
	}

	var errs utils.OutputErrors
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}

	return errs.PrintErrors()
}
