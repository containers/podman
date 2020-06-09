package network

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/parse"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networkReloadDescription = `reload container networks, recreating firewall rules`
	networkReloadCommand     = &cobra.Command{
		Use:   "reload [flags] CONTAINER [CONTAINER...]",
		Short: "Reload firewall rules for one or more containers",
		Long:  networkReloadDescription,
		RunE:  networkReload,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman network reload --latest
  podman network reload 3c13ef6dd843
  podman network reload test1 test2`,
	}
)

var (
	reloadOptions entities.NetworkReloadOptions
)

func reloadFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&reloadOptions.All, "all", "a", false, "Reload networks of all containers")
	flags.BoolVarP(&reloadOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: networkReloadCommand,
		Parent:  networkCmd,
	})
	flags := networkReloadCommand.Flags()
	reloadFlags(flags)
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
