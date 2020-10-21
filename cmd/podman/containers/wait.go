package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	waitDescription = `Block until one or more containers stop and then print their exit codes.
`
	waitCommand = &cobra.Command{
		Use:   "wait [options] CONTAINER [CONTAINER...]",
		Short: "Block on one or more containers",
		Long:  waitDescription,
		RunE:  wait,
		Example: `podman wait --interval 5s ctrID
  podman wait ctrID1 ctrID2`,
	}

	containerWaitCommand = &cobra.Command{
		Use:   waitCommand.Use,
		Short: waitCommand.Short,
		Long:  waitCommand.Long,
		RunE:  waitCommand.RunE,
		Example: `podman container wait --interval 5s ctrID
  podman container wait ctrID1 ctrID2`,
	}
)

var (
	waitOptions   = entities.WaitOptions{}
	waitCondition string
	waitInterval  string
)

func waitFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&waitInterval, "interval", "i", "250ns", "Time Interval to wait before polling for completion")
	flags.StringVar(&waitCondition, "condition", "stopped", "Condition to wait on")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: waitCommand,
	})
	waitFlags(waitCommand.Flags())
	validate.AddLatestFlag(waitCommand, &waitOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerWaitCommand,
		Parent:  containerCmd,
	})
	waitFlags(containerWaitCommand.Flags())
	validate.AddLatestFlag(containerWaitCommand, &waitOptions.Latest)

}

func wait(cmd *cobra.Command, args []string) error {
	var (
		err  error
		errs utils.OutputErrors
	)
	if waitOptions.Interval, err = time.ParseDuration(waitInterval); err != nil {
		var err1 error
		if waitOptions.Interval, err1 = time.ParseDuration(waitInterval + "ms"); err1 != nil {
			return err
		}
	}

	if !waitOptions.Latest && len(args) == 0 {
		return errors.Errorf("%q requires a name, id, or the \"--latest\" flag", cmd.CommandPath())
	}
	if waitOptions.Latest && len(args) > 0 {
		return errors.New("--latest and containers cannot be used together")
	}

	waitOptions.Condition, err = define.StringToContainerStatus(waitCondition)
	if err != nil {
		return err
	}

	responses, err := registry.ContainerEngine().ContainerWait(context.Background(), args, waitOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		if r.Error == nil {
			fmt.Println(r.ExitCode)
		} else {
			errs = append(errs, r.Error)
		}
	}
	return errs.PrintErrors()
}
