package containers

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	unpauseDescription = `Unpauses one or more previously paused containers.  The container name or ID can be used.`
	unpauseCommand     = &cobra.Command{
		Use:               "unpause [flags] CONTAINER [CONTAINER...]",
		Short:             "Unpause the processes in one or more containers",
		Long:              unpauseDescription,
		RunE:              unpause,
		PersistentPreRunE: preRunE,
		Example: `podman unpause ctrID
  podman unpause --all`,
	}
	unPauseOptions = entities.PauseUnPauseOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: unpauseCommand,
		Parent:  containerCmd,
	})
	flags := unpauseCommand.Flags()
	flags.BoolVarP(&unPauseOptions.All, "all", "a", false, "Pause all running containers")
}

func unpause(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if rootless.IsRootless() && !registry.IsRemote() {
		return errors.New("unpause is not supported for rootless containers")
	}
	if len(args) < 1 && !unPauseOptions.All {
		return errors.Errorf("you must provide at least one container name or id")
	}
	responses, err := registry.ContainerEngine().ContainerUnpause(context.Background(), args, unPauseOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}
