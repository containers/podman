package containers

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
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
	pruneDescription = fmt.Sprintf(`podman container prune

	Removes all non running containers`)
	pruneCommand = &cobra.Command{
		Use:     "prune [options]",
		Short:   "Remove all non running containers",
		Long:    pruneDescription,
		RunE:    prune,
		Example: `podman container prune`,
		Args:    validate.NoArgs,
	}
	force  bool
	filter = []string{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pruneCommand,
		Parent:  containerCmd,
	})
	flags := pruneCommand.Flags()
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation.  The default is false")
	flags.StringArrayVar(&filter, "filter", []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
}

func prune(cmd *cobra.Command, args []string) error {
	var (
		pruneOptions = entities.ContainerPruneOptions{}
	)
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all non running containers.")
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	// TODO Remove once filter refactor is finished and url.Values done.
	for _, f := range filter {
		t := strings.SplitN(f, "=", 2)
		pruneOptions.Filters = make(url.Values)
		if len(t) < 2 {
			return errors.Errorf("filter input must be in the form of filter=value: %s is invalid", f)
		}
		pruneOptions.Filters.Add(t[0], t[1])
	}
	responses, err := registry.ContainerEngine().ContainerPrune(context.Background(), pruneOptions)

	if err != nil {
		return err
	}
	return utils.PrintContainerPruneResults(responses)
}
