package containers

import (
	"context"
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	restartDescription = fmt.Sprintf(`Restarts one or more running containers. The container ID or name can be used.

  A timeout before forcibly stopping can be set, but defaults to %d seconds.`, containerConfig.Engine.StopTimeout)

	restartCommand = &cobra.Command{
		Use:   "restart [options] CONTAINER [CONTAINER...]",
		Short: "Restart one or more containers",
		Long:  restartDescription,
		RunE:  restart,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman restart ctrID
  podman restart --latest
  podman restart ctrID1 ctrID2`,
	}

	containerRestartCommand = &cobra.Command{
		Use:               restartCommand.Use,
		Short:             restartCommand.Short,
		Long:              restartCommand.Long,
		RunE:              restartCommand.RunE,
		Args:              restartCommand.Args,
		ValidArgsFunction: restartCommand.ValidArgsFunction,
		Example: `podman container restart ctrID
  podman container restart --latest
  podman container restart ctrID1 ctrID2`,
	}
)

var (
	restartOptions = entities.RestartOptions{}
	restartTimeout uint
)

func restartFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&restartOptions.All, "all", "a", false, "Restart all non-running containers")
	flags.BoolVar(&restartOptions.Running, "running", false, "Restart only running containers when --all is used")

	timeFlagName := "time"
	flags.UintVarP(&restartTimeout, timeFlagName, "t", containerConfig.Engine.StopTimeout, "Seconds to wait for stop before killing the container")
	_ = cmd.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: restartCommand,
	})
	restartFlags(restartCommand)
	validate.AddLatestFlag(restartCommand, &restartOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerRestartCommand,
		Parent:  containerCmd,
	})
	restartFlags(containerRestartCommand)
	validate.AddLatestFlag(containerRestartCommand, &restartOptions.Latest)
}

func restart(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if len(args) < 1 && !restartOptions.Latest && !restartOptions.All {
		return errors.Wrapf(define.ErrInvalidArg, "you must provide at least one container name or ID")
	}
	if len(args) > 0 && restartOptions.Latest {
		return errors.Wrapf(define.ErrInvalidArg, "--latest and containers cannot be used together")
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
