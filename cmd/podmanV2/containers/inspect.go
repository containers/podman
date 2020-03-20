package containers

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// podman container _inspect_
	inspectCmd = &cobra.Command{
		Use:     "inspect [flags] CONTAINER",
		Short:   "Display the configuration of a container",
		Long:    `Displays the low-level information on a container identified by name or ID.`,
		PreRunE: inspectPreRunE,
		RunE:    inspect,
		Example: `podman container inspect myCtr
  podman container inspect -l --format '{{.Id}} {{.Config.Labels}}'`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  containerCmd,
	})
}

func inspectPreRunE(cmd *cobra.Command, args []string) (err error) {
	err = preRunE(cmd, args)
	if err != nil {
		return
	}

	_, err = registry.NewImageEngine(cmd, args)
	return err
}

func inspect(cmd *cobra.Command, args []string) error {
	return nil
}
