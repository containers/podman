package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	pruneOptions = entities.SystemPruneOptions{}

	pruneDescription = fmt.Sprintf(`
	podman system prune

        Remove unused data
`)

	pruneCommand = &cobra.Command{
		Use:     "prune [options]",
		Short:   "Remove unused data",
		Args:    validate.NoArgs,
		Long:    pruneDescription,
		RunE:    prune,
		Example: `podman system prune`,
	}
	force bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pruneCommand,
		Parent:  systemCmd,
	})
	flags := pruneCommand.Flags()
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation.  The default is false")
	flags.BoolVarP(&pruneOptions.All, "all", "a", false, "Remove all unused data")
	flags.BoolVar(&pruneOptions.Volume, "volumes", false, "Prune volumes")

}

func prune(cmd *cobra.Command, args []string) error {
	// Prompt for confirmation if --force is not set
	if !force {
		reader := bufio.NewReader(os.Stdin)
		volumeString := ""
		if pruneOptions.Volume {
			volumeString = `
        - all volumes not used by at least one container`
		}
		fmt.Printf(`
WARNING! This will remove:
        - all stopped containers%s
        - all stopped pods
        - all dangling images
        - all build cache
Are you sure you want to continue? [y/N] `, volumeString)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	// TODO: support for filters in system prune
	response, err := registry.ContainerEngine().SystemPrune(context.Background(), pruneOptions)
	if err != nil {
		return err
	}
	// Print pod prune results
	fmt.Println("Deleted Pods")
	err = utils.PrintPodPruneResults(response.PodPruneReport)
	if err != nil {
		return err
	}
	// Print container prune results
	fmt.Println("Deleted Containers")
	err = utils.PrintContainerPruneResults(response.ContainerPruneReport)
	if err != nil {
		return err
	}
	// Print Volume prune results
	if pruneOptions.Volume {
		fmt.Println("Deleted Volumes")
		err = utils.PrintVolumePruneResults(response.VolumePruneReport)
		if err != nil {
			return err
		}
	}
	// Print Images prune results
	fmt.Println("Deleted Images")
	return utils.PrintImagePruneResults(response.ImagePruneReport)
}
