package pods

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	podRestartDescription = `The pod ID or name can be used.

  All of the containers within each of the specified pods will be restarted. If a container in a pod is not currently running it will be started.`
	restartCommand = &cobra.Command{
		Use:   "restart [flags] POD [POD...]",
		Short: "Restart one or more pods",
		Long:  podRestartDescription,
		RunE:  restart,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman pod restart podID1 podID2
  podman pod restart --latest
  podman pod restart --all`,
	}
)

var (
	restartOptions = entities.PodRestartOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: restartCommand,
		Parent:  podCmd,
	})

	flags := restartCommand.Flags()
	flags.BoolVarP(&restartOptions.All, "all", "a", false, "Restart all running pods")
	flags.BoolVarP(&restartOptions.Latest, "latest", "l", false, "Restart the latest pod podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func restart(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	responses, err := registry.ContainerEngine().PodRestart(context.Background(), args, restartOptions)
	if err != nil {
		return err
	}
	// in the cli, first we print out all the successful attempts
	for _, r := range responses {
		if len(r.Errs) == 0 {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Errs...)
		}
	}
	return errs.PrintErrors()
}
