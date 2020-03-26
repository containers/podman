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
	podPauseDescription = `The pod name or ID can be used.

  All running containers within each specified pod will then be paused.`
	pauseCommand = &cobra.Command{
		Use:   "pause [flags] POD [POD...]",
		Short: "Pause one or more pods",
		Long:  podPauseDescription,
		RunE:  pause,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman pod pause podID1 podID2
  podman pod pause --latest
  podman pod pause --all`,
	}
)

var (
	pauseOptions entities.PodPauseOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: pauseCommand,
		Parent:  podCmd,
	})
	flags := pauseCommand.Flags()
	flags.BoolVarP(&pauseOptions.All, "all", "a", false, "Pause all running pods")
	flags.BoolVarP(&pauseOptions.Latest, "latest", "l", false, "Act on the latest pod podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}
func pause(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	responses, err := registry.ContainerEngine().PodPause(context.Background(), args, pauseOptions)
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
