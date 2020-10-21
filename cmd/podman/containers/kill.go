package containers

import (
	"context"
	"errors"
	"fmt"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/signal"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	killDescription = "The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal."
	killCommand     = &cobra.Command{
		Use:   "kill [options] CONTAINER [CONTAINER...]",
		Short: "Kill one or more running containers with a specific signal",
		Long:  killDescription,
		RunE:  kill,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman kill mywebserver
  podman kill 860a4b23
  podman kill --signal TERM ctrID`,
	}

	containerKillCommand = &cobra.Command{
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, false)
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
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: killCommand,
	})
	killFlags(killCommand.Flags())
	validate.AddLatestFlag(killCommand, &killOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerKillCommand,
		Parent:  containerCmd,
	})
	killFlags(containerKillCommand.Flags())
	validate.AddLatestFlag(containerKillCommand, &killOptions.Latest)
}

func kill(_ *cobra.Command, args []string) error {
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
