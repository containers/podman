package images

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgenutil"
	"github.com/spf13/cobra"
)

var (
	buildPruneDescription = `Remove build cache.`
	buildPruneCmd         = &cobra.Command{
		Use:               "prune [options]",
		Args:              validate.NoArgs,
		Short:             "Remove unused builder images",
		Long:              pruneDescription,
		RunE:              buildPrune,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman image prune`,
	}
	buildPruneOpts = entities.ImagePruneOptions{}
	buildForce     bool
	buildFilter    = []string{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildPruneCmd,
		Parent:  buildxCmd,
	})
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildPruneCmd,
		Parent:  buildCmd,
	})

	flags := buildPruneCmd.Flags()
	flags.BoolVarP(&pruneOpts.All, "all", "a", false, "Remove all unused build cache, not just dangling ones")
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation")

	filterFlagName := "filter"
	flags.StringArrayVar(&filter, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	_ = buildPruneCmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePruneFilters)
}

func buildPrune(cmd *cobra.Command, args []string) error {
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("%s", buildCreatePruneWarningMessage(buildPruneOpts))
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	filterMap, err := specgenutil.ParseFilters(filter)
	if err != nil {
		return err
	}
	for k, v := range filterMap {
		for _, val := range v {
			pruneOpts.Filter = append(pruneOpts.Filter, fmt.Sprintf("%s=%s", k, val))
		}
	}
	results, err := registry.ImageEngine().Prune(registry.GetContext(), pruneOpts)
	if err != nil {
		return err
	}

	return utils.PrintImagePruneResults(results, false)
}

func buildCreatePruneWarningMessage(pruneOpts entities.ImagePruneOptions) string {
	question := "Are you sure you want to continue? [y/N] "
	warning := "WARNING! This will remove all dangling build cache.\n"
	if pruneOpts.All {
		warning = "WARNING! This will remove all build cache.\n"
	}
	return warning + question
}
