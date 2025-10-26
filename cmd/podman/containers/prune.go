package containers

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/podman/v6/cmd/podman/common"
	"github.com/containers/podman/v6/cmd/podman/parse"
	"github.com/containers/podman/v6/cmd/podman/registry"
	"github.com/containers/podman/v6/cmd/podman/utils"
	"github.com/containers/podman/v6/cmd/podman/validate"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
)

var (
	pruneDescription = `podman container prune

	Removes containers`
	pruneCommand = &cobra.Command{
		Use:               "prune [options]",
		Short:             "Remove containers",
		Long:              pruneDescription,
		RunE:              prune,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman container prune`,
		Args:              validate.NoArgs,
	}
	force  bool
	pruneRunning    bool
	filter = []string{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCommand,
		Parent:  containerCmd,
	})
	flags := pruneCommand.Flags()
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation.  The default is false")
	flags.BoolVarP(&pruneRunning, "all", "a", false, "Remove running containers. The default is to remove only non running containers")
	filterFlagName := "filter"
	flags.StringArrayVar(&filter, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	_ = pruneCommand.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePruneFilters)
}

func prune(_ *cobra.Command, _ []string) error {
	var (
		pruneOptions = entities.ContainerPruneOptions{}
		err          error
	)
	if !force {
		reader := bufio.NewReader(os.Stdin)
		if pruneRunning {
			fmt.Println("WARNING! This will remove ALL containers.")
		} else {
			fmt.Println("WARNING! This will remove all non running containers.")
		}
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	pruneOptions.PruneRunning = pruneRunning
	pruneOptions.Filters, err = parse.FilterArgumentsIntoFilters(filter)
	if err != nil {
		return err
	}
	responses, err := registry.ContainerEngine().ContainerPrune(context.Background(), pruneOptions)

	if err != nil {
		return err
	}
	return utils.PrintContainerPruneResults(responses, false)
}
