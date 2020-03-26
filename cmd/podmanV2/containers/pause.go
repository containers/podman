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
	pauseDescription = `Pauses one or more running containers.  The container name or ID can be used.`
	pauseCommand     = &cobra.Command{
		Use:               "pause [flags] CONTAINER [CONTAINER...]",
		Short:             "Pause all the processes in one or more containers",
		Long:              pauseDescription,
		RunE:              pause,
		PersistentPreRunE: preRunE,
		Example: `podman pause mywebserver
  podman pause 860a4b23
  podman pause -a`,
	}

	pauseOpts = entities.PauseUnPauseOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pauseCommand,
	})
	flags := pauseCommand.Flags()
	flags.BoolVarP(&pauseOpts.All, "all", "a", false, "Pause all running containers")
	pauseCommand.SetHelpTemplate(registry.HelpTemplate())
	pauseCommand.SetUsageTemplate(registry.UsageTemplate())
}

func pause(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if rootless.IsRootless() && !registry.IsRemote() {
		return errors.New("pause is not supported for rootless containers")
	}
	if len(args) < 1 && !pauseOpts.All {
		return errors.Errorf("you must provide at least one container name or id")
	}
	responses, err := registry.ContainerEngine().ContainerPause(context.Background(), args, pauseOpts)
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
