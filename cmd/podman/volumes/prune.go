package volumes

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
	"github.com/spf13/cobra"
)

var (
	volumePruneDescription = `Volumes that are not currently owned by a container will be removed.

  The command prompts for confirmation which can be overridden with the --force flag.
  Note all data will be destroyed.`
	pruneCommand = &cobra.Command{
		Use:               "prune [options]",
		Args:              validate.NoArgs,
		Short:             "Remove all unused volumes",
		Long:              volumePruneDescription,
		RunE:              prune,
		ValidArgsFunction: completion.AutocompleteNone,
	}
	filter = []string{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCommand,
		Parent:  volumeCmd,
	})
	flags := pruneCommand.Flags()

	filterFlagName := "filter"
	flags.StringArrayVar(&filter, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	_ = pruneCommand.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteVolumeFilters)
	flags.BoolP("force", "f", false, "Do not prompt for confirmation")
}

func prune(cmd *cobra.Command, args []string) error {
	var (
		pruneOptions  = entities.VolumePruneOptions{}
		listOptions   = entities.VolumeListOptions{}
		unusedOptions = entities.VolumeListOptions{}
	)
	// Prompt for confirmation if --force is not set
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	pruneOptions.Filters, err = parse.FilterArgumentsIntoFilters(filter)
	if err != nil {
		return err
	}
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all volumes not used by at least one container. The following volumes will be removed:")
		if err != nil {
			return err
		}
		listOptions.Filter, err = parse.FilterArgumentsIntoFilters(filter)
		if err != nil {
			return err
		}
		// filter all the dangling volumes
		unusedOptions.Filter = make(map[string][]string, 1)
		unusedOptions.Filter["dangling"] = []string{"true"}
		unusedVolumes, err := registry.ContainerEngine().VolumeList(context.Background(), unusedOptions)
		if err != nil {
			return err
		}
		// filter volumes based on user input
		filteredVolumes, err := registry.ContainerEngine().VolumeList(context.Background(), listOptions)
		if err != nil {
			return err
		}
		finalVolumes := getIntersection(unusedVolumes, filteredVolumes)
		if len(finalVolumes) < 1 {
			fmt.Println("No dangling volumes found")
			return nil
		}
		for _, fv := range finalVolumes {
			fmt.Println(fv.Name)
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
	responses, err := registry.ContainerEngine().VolumePrune(context.Background(), pruneOptions)
	if err != nil {
		return err
	}
	return utils.PrintVolumePruneResults(responses, false)
}

func getIntersection(a, b []*entities.VolumeListReport) []*entities.VolumeListReport {
	var intersection []*entities.VolumeListReport
	hash := make(map[string]bool, len(a))
	for _, aa := range a {
		hash[aa.Name] = true
	}
	for _, bb := range b {
		if hash[bb.Name] {
			intersection = append(intersection, bb)
		}
	}
	return intersection
}
