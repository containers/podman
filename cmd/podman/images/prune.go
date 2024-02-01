package images

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/parse"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	pruneDescription = `Removes dangling or unused images from local storage.`
	pruneCmd         = &cobra.Command{
		Use:               "prune [options]",
		Args:              validate.NoArgs,
		Short:             "Remove unused images",
		Long:              pruneDescription,
		RunE:              prune,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman image prune`,
	}

	pruneOpts = entities.ImagePruneOptions{}
	force     bool
	filter    = []string{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCmd,
		Parent:  buildxCmd,
	})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCmd,
		Parent:  imageCmd,
	})

	flags := pruneCmd.Flags()
	flags.BoolVarP(&pruneOpts.All, "all", "a", false, "Remove all images not in use by containers, not just dangling ones")
	flags.BoolVarP(&pruneOpts.External, "external", "", false, "Remove images even when they are used by external containers (e.g., by build containers)")
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation")

	filterFlagName := "filter"
	flags.StringArrayVar(&filter, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	_ = pruneCmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePruneFilters)
}

func prune(cmd *cobra.Command, args []string) error {
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("%s", createPruneWarningMessage(pruneOpts))
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	filterMap, err := parse.FilterArgumentsIntoFilters(filter)
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

func createPruneWarningMessage(pruneOpts entities.ImagePruneOptions) string {
	question := "Are you sure you want to continue? [y/N] "
	if pruneOpts.All {
		return "WARNING! This command removes all images without at least one container associated with them.\n" + question
	}
	return "WARNING! This command removes all dangling images.\n" + question
}
