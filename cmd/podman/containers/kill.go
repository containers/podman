package containers

import (
	"context"
	"errors"
	"fmt"

	"github.com/containers/libpod/v2/cmd/podman/parse"
	"github.com/containers/libpod/v2/cmd/podman/registry"
	"github.com/containers/libpod/v2/cmd/podman/utils"
	"github.com/containers/libpod/v2/pkg/domain/entities"
	"github.com/containers/libpod/v2/pkg/signal"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	killDescription = "The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal."
	killCommand     = &cobra.Command{
		Use:   "kill [flags] CONTAINER [CONTAINER...]",
		Short: "Kill one or more running containers with a specific signal",
		Long:  killDescription,
		RunE:  kill,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman kill mywebserver
  podman kill 860a4b23
  podman kill --signal TERM ctrID`,
	}

	containerKillCommand = &cobra.Command{
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Use:   killCommand.Use,
		Short: killCommand.Short,
		Long:  killCommand.Long,
		RunE:  killCommand.RunE,
		Example: `podman container kill mywebserver
  podman container kill 860a4b23
  podman container kill --signal TERM ctrID`,
	}
)

var (
	killOptions = entities.KillOptions{}
)

func killFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&killOptions.All, "all", "a", false, "Signal all running containers")
	flags.StringVarP(&killOptions.Signal, "signal", "s", "KILL", "Signal to send to the container")
	flags.BoolVarP(&killOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: killCommand,
	})
	flags := killCommand.Flags()
	killFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerKillCommand,
		Parent:  containerCmd,
	})

	containerKillFlags := containerKillCommand.Flags()
	killFlags(containerKillFlags)
}

func kill(cmd *cobra.Command, args []string) error {
	var (
		err  error
		errs utils.OutputErrors
	)
	// Check if the signalString provided by the user is valid
	// Invalid signals will return err
	sig, err := signal.ParseSignalNameOrNumber(killOptions.Signal)
	if err != nil {
		return err
	}
	if sig < 1 || sig > 64 {
		return errors.New("valid signals are 1 through 64")
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
