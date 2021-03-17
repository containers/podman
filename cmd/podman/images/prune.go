package images

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	pruneDescription = `Removes all unnamed images from local storage.

  If an image is not being used by a container, it will be removed from the system.`
	pruneCmd = &cobra.Command{
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
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pruneCmd,
		Parent:  imageCmd,
	})

	flags := pruneCmd.Flags()
	flags.BoolVarP(&pruneOpts.All, "all", "a", false, "Remove all unused images, not just dangling ones")
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation")

	filterFlagName := "filter"
	flags.StringArrayVar(&filter, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	//TODO: add completion for filters
	_ = pruneCmd.RegisterFlagCompletionFunc(filterFlagName, completion.AutocompleteNone)
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

	results, err := registry.ImageEngine().Prune(registry.GetContext(), pruneOpts)
	if err != nil {
		return err
	}

	return utils.PrintImagePruneResults(results, false)
}

func createPruneWarningMessage(pruneOpts entities.ImagePruneOptions) string {
	question := "Are you sure you want to continue? [y/N] "
	if pruneOpts.All {
		return "WARNING! This will remove all images without at least one container associated to them.\n" + question
	}
	return "WARNING! This will remove all dangling images.\n" + question
}
