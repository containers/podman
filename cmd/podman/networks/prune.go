package network

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networkPruneDescription = `Prune unused networks`
	networkPruneCommand     = &cobra.Command{
		Use:               "prune [options]",
		Short:             "network prune",
		Long:              networkPruneDescription,
		RunE:              networkPrune,
		Example:           `podman network prune`,
		Args:              validate.NoArgs,
		ValidArgsFunction: common.AutocompleteNetworks,
	}
)

var (
	networkPruneOptions entities.NetworkPruneOptions
	force               bool
)

func networkPruneFlags(flags *pflag.FlagSet) {
	//TODO: Not implemented but for future reference
	//flags.StringSliceVar(&networkPruneOptions.Filters,"filters", []string{}, "provide filter values (e.g. 'until=<timestamp>')")
	flags.BoolVarP(&force, "force", "f", false, "do not prompt for confirmation")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: networkPruneCommand,
		Parent:  networkCmd,
	})
	flags := networkPruneCommand.Flags()
	networkPruneFlags(flags)
}

func networkPrune(cmd *cobra.Command, _ []string) error {
	var (
		errs utils.OutputErrors
	)
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all networks not used by at least one container.")
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	responses, err := registry.ContainerEngine().NetworkPrune(registry.Context(), networkPruneOptions)
	if err != nil {
		setExitCode(err)
		return err
	}
	for _, r := range responses {
		if r.Error == nil {
			fmt.Println(r.Name)
		} else {
			setExitCode(r.Error)
			errs = append(errs, r.Error)
		}
	}
	return errs.PrintErrors()
}
