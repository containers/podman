package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/parse"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

var (
	pruneOptions     = entities.SystemPruneOptions{}
	filters          []string
	pruneDescription = `
	podman system prune

        Remove unused data
`

	pruneCommand = &cobra.Command{
		Use:               "prune [options]",
		Short:             "Remove unused data",
		Args:              validate.NoArgs,
		Long:              pruneDescription,
		RunE:              prune,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman system prune`,
	}
	force bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCommand,
		Parent:  systemCmd,
	})
	flags := pruneCommand.Flags()
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation.  The default is false")
	flags.BoolVarP(&pruneOptions.All, "all", "a", false, "Remove all unused data")
	flags.BoolVar(&pruneOptions.Volume, "volumes", false, "Prune volumes")
	filterFlagName := "filter"
	flags.StringArrayVar(&filters, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	_ = pruneCommand.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePruneFilters)
}

func prune(cmd *cobra.Command, args []string) error {
	var err error
	// Prompt for confirmation if --force is not set
	if !force {
		reader := bufio.NewReader(os.Stdin)
		volumeString := ""
		if pruneOptions.Volume {
			volumeString = `
	- all volumes not used by at least one container`
		}

		fmt.Printf(createPruneWarningMessage(pruneOptions), volumeString, "Are you sure you want to continue? [y/N] ")

		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	pruneOptions.Filters, err = parse.FilterArgumentsIntoFilters(filters)
	if err != nil {
		return err
	}

	response, err := registry.ContainerEngine().SystemPrune(context.Background(), pruneOptions)
	if err != nil {
		return err
	}
	// Print container prune results
	err = utils.PrintContainerPruneResults(response.ContainerPruneReports, true)
	if err != nil {
		return err
	}
	// Print pod prune results
	err = utils.PrintPodPruneResults(response.PodPruneReport, true)
	if err != nil {
		return err
	}
	// Print Volume prune results
	if pruneOptions.Volume {
		err = utils.PrintVolumePruneResults(response.VolumePruneReports, true)
		if err != nil {
			return err
		}
	}
	// Print Images prune results
	err = utils.PrintImagePruneResults(response.ImagePruneReports, true)
	if err != nil {
		return err
	}

	fmt.Printf("Total reclaimed space: %s\n", units.HumanSize((float64)(response.ReclaimedSpace)))
	return nil
}

func createPruneWarningMessage(pruneOpts entities.SystemPruneOptions) string {
	if pruneOpts.All {
		return `WARNING! This command removes:
	- all stopped containers
	- all networks not used by at least one container%s
	- all images without at least one container associated with them
	- all build cache

%s`
	}
	return `WARNING! This command removes:
	- all stopped containers
	- all networks not used by at least one container%s
	- all dangling images
	- all dangling build cache

%s`
}
