package volumes

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
	volumePruneDescription = `Volumes that are not currently owned by a container will be removed.

  The command prompts for confirmation which can be overridden with the --force flag.
  Note all data will be destroyed.`
	pruneCommand = &cobra.Command{
		Use:   "prune [options]",
		Args:  validate.NoArgs,
		Short: "Remove all unused volumes",
		Long:  volumePruneDescription,
		RunE:  prune,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pruneCommand,
		Parent:  volumeCmd,
	})
	flags := pruneCommand.Flags()
	flags.BoolP("force", "f", false, "Do not prompt for confirmation")
}

func prune(cmd *cobra.Command, args []string) error {
	// Prompt for confirmation if --force is not set
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all volumes not used by at least one container.")
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	responses, err := registry.ContainerEngine().VolumePrune(context.Background())
	if err != nil {
		return err
	}
	return utils.PrintVolumePruneResults(responses)
}
