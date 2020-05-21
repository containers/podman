package images

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/utils"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	pruneDescription = `Removes all unnamed images from local storage.

  If an image is not being used by a container, it will be removed from the system.`
	pruneCmd = &cobra.Command{
		Use:     "prune",
		Args:    validate.NoArgs,
		Short:   "Remove unused images",
		Long:    pruneDescription,
		RunE:    prune,
		Example: `podman image prune`,
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
	flags.StringArrayVar(&filter, "filter", []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")

}

func prune(cmd *cobra.Command, args []string) error {
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf(`
WARNING! This will remove all dangling images.
Are you sure you want to continue? [y/N] `)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	results, err := registry.ImageEngine().Prune(registry.GetContext(), pruneOpts)
	if err != nil {
		return err
	}

	return utils.PrintImagePruneResults(results)
}
