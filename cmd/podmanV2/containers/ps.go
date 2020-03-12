package containers

import (
	"strings"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// podman _ps_
	psCmd = &cobra.Command{
		Use:               "ps",
		Args:              cobra.NoArgs,
		Short:             listCmd.Short,
		Long:              listCmd.Long,
		PersistentPreRunE: preRunE,
		RunE:              containers,
		Example:           strings.Replace(listCmd.Example, "container list", "ps", -1),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: psCmd,
	})
}
