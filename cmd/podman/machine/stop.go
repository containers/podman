package machine

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	stopCmd = &cobra.Command{
		Use:               "stop NAME",
		Short:             "Stop an existing machine",
		Long:              "Stop an existing machine ",
		RunE:              stop,
		Args:              cobra.ExactArgs(1),
		Example:           `podman machine stop myvm`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: stopCmd,
		Parent:  machineCmd,
	})
}

func stop(cmd *cobra.Command, args []string) error {
	test := new(TestVM)
	test.Stop(args[0])
	return nil
}
