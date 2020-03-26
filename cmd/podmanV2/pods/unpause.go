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
	podUnpauseDescription = `The podman unpause command will unpause all "paused" containers assigned to the pod.

  The pod name or ID can be used.`
	unpauseCommand = &cobra.Command{
		Use:   "unpause [flags] POD [POD...]",
		Short: "Unpause one or more pods",
		Long:  podUnpauseDescription,
		RunE:  unpause,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman pod unpause podID1 podID2
  podman pod unpause --all
  podman pod unpause --latest`,
	}
)

var (
	unpauseOptions entities.PodunpauseOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: unpauseCommand,
		Parent:  podCmd,
	})
	flags := unpauseCommand.Flags()
	flags.BoolVarP(&unpauseOptions.All, "all", "a", false, "Pause all running pods")
	flags.BoolVarP(&unpauseOptions.Latest, "latest", "l", false, "Act on the latest pod podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}
func unpause(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	responses, err := registry.ContainerEngine().PodUnpause(context.Background(), args, unpauseOptions)
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
