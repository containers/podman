package containers

import (
	"context"
	"fmt"

	"github.com/containers/libpod/v2/cmd/podman/parse"
	"github.com/containers/libpod/v2/cmd/podman/registry"
	"github.com/containers/libpod/v2/cmd/podman/utils"
	"github.com/containers/libpod/v2/libpod/define"
	"github.com/containers/libpod/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	restartDescription = fmt.Sprintf(`Restarts one or more running containers. The container ID or name can be used.

  A timeout before forcibly stopping can be set, but defaults to %d seconds.`, containerConfig.Engine.StopTimeout)

	restartCommand = &cobra.Command{
		Use:   "restart [flags] CONTAINER [CONTAINER...]",
		Short: "Restart one or more containers",
		Long:  restartDescription,
		RunE:  restart,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman restart ctrID
  podman restart --latest
  podman restart ctrID1 ctrID2`,
	}

	containerRestartCommand = &cobra.Command{
		Use:   restartCommand.Use,
		Short: restartCommand.Short,
		Long:  restartCommand.Long,
		RunE:  restartCommand.RunE,
		Example: `podman container restart ctrID
  podman container restart --latest
  podman container restart ctrID1 ctrID2`,
	}
)

var (
	restartOptions = entities.RestartOptions{}
	restartTimeout uint
)

func restartFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&restartOptions.All, "all", "a", false, "Restart all non-running containers")
	flags.BoolVarP(&restartOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&restartOptions.Running, "running", false, "Restart only running containers when --all is used")
	flags.UintVarP(&restartTimeout, "time", "t", containerConfig.Engine.StopTimeout, "Seconds to wait for stop before killing the container")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
	flags.SetNormalizeFunc(utils.AliasFlags)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: restartCommand,
	})
	flags := restartCommand.Flags()
	restartFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerRestartCommand,
		Parent:  containerCmd,
	})

	containerRestartFlags := containerRestartCommand.Flags()
	restartFlags(containerRestartFlags)
}

func restart(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if len(args) < 1 && !restartOptions.Latest && !restartOptions.All {
		return errors.Wrapf(define.ErrInvalidArg, "you must provide at least one container name or ID")
	}

	if cmd.Flag("time").Changed {
		restartOptions.Timeout = &restartTimeout
	}
	responses, err := registry.ContainerEngine().ContainerRestart(context.Background(), args, restartOptions)
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
