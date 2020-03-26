package containers

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/signal"
	"github.com/spf13/cobra"
)

var (
	killDescription = "The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal."
	killCommand     = &cobra.Command{
		Use:               "kill [flags] CONTAINER [CONTAINER...]",
		Short:             "Kill one or more running containers with a specific signal",
		Long:              killDescription,
		RunE:              kill,
		PersistentPreRunE: preRunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman kill mywebserver
  podman kill 860a4b23
  podman kill --signal TERM ctrID`,
	}
)

var (
	killOptions = entities.KillOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: killCommand,
	})
	flags := killCommand.Flags()
	flags.BoolVarP(&killOptions.All, "all", "a", false, "Signal all running containers")
	flags.StringVarP(&killOptions.Signal, "signal", "s", "KILL", "Signal to send to the container")
	flags.BoolVarP(&killOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func kill(cmd *cobra.Command, args []string) error {
	var (
		err  error
		errs utils.OutputErrors
	)
	// Check if the signalString provided by the user is valid
	// Invalid signals will return err
	if _, err = signal.ParseSignalNameOrNumber(killOptions.Signal); err != nil {
		return err
	}
	responses, err := registry.ContainerEngine().ContainerKill(context.Background(), args, killOptions)
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
