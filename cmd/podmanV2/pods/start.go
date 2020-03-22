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
	podStartDescription = `The pod name or ID can be used.

  All containers defined in the pod will be started.`
	startCommand = &cobra.Command{
		Use:   "start [flags] POD [POD...]",
		Short: "Start one or more pods",
		Long:  podStartDescription,
		RunE:  start,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman pod start podID
  podman pod start --latest
  podman pod start --all`,
	}
)

var (
	startOptions = entities.PodStartOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: startCommand,
		Parent:  podCmd,
	})

	flags := startCommand.Flags()
	flags.BoolVarP(&startOptions.All, "all", "a", false, "Restart all running pods")
	flags.BoolVarP(&startOptions.Latest, "latest", "l", false, "Restart the latest pod podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func start(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	responses, err := registry.ContainerEngine().PodStart(context.Background(), args, startOptions)
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
