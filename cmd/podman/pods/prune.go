package pods

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	pruneOptions = entities.PodPruneOptions{}
)

var (
	pruneDescription = `podman pod prune Removes all exited pods`

	pruneCommand = &cobra.Command{
		Use:               "prune [options]",
		Args:              validate.NoArgs,
		Short:             "Remove all stopped pods and their containers",
		Long:              pruneDescription,
		RunE:              prune,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman pod prune`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCommand,
		Parent:  podCmd,
	})
	flags := pruneCommand.Flags()
	flags.BoolVarP(&pruneOptions.Force, "force", "f", false, "Do not prompt for confirmation.  The default is false")
}

func prune(cmd *cobra.Command, args []string) error {
	if !pruneOptions.Force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all stopped/exited pods..")
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	responses, err := registry.ContainerEngine().PodPrune(context.Background(), pruneOptions)
	if err != nil {
		return err
	}
	return utils.PrintPodPruneResults(responses, false)
}
