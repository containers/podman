package containers

import (
	"context"
	"fmt"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	stopDescription = fmt.Sprintf(`Stops one or more running containers.  The container name or ID can be used.

  A timeout to forcibly stop the container can also be set but defaults to %d seconds otherwise.`, containerConfig.Engine.StopTimeout)
	stopCommand = &cobra.Command{
		Use:   "stop [options] CONTAINER [CONTAINER...]",
		Short: "Stop one or more containers",
		Long:  stopDescription,
		RunE:  stop,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, true)
		},
		Example: `podman stop ctrID
  podman stop --latest
  podman stop --time 2 mywebserver 6e534f14da9d`,
	}

	containerStopCommand = &cobra.Command{
		Use:   stopCommand.Use,
		Short: stopCommand.Short,
		Long:  stopCommand.Long,
		RunE:  stopCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, true)
		},
		Example: `podman container stop ctrID
  podman container stop --latest
  podman container stop --time 2 mywebserver 6e534f14da9d`,
	}
)

var (
	stopOptions = entities.StopOptions{}
	stopTimeout uint
)

func stopFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&stopOptions.All, "all", "a", false, "Stop all running containers")
	flags.BoolVarP(&stopOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified container is missing")
	flags.StringArrayVarP(&stopOptions.CIDFiles, "cidfile", "", nil, "Read the container ID from the file")
	flags.UintVarP(&stopTimeout, "time", "t", containerConfig.Engine.StopTimeout, "Seconds to wait for stop before killing the container")

	if registry.IsRemote() {
		_ = flags.MarkHidden("cidfile")
		_ = flags.MarkHidden("ignore")
	}
	flags.SetNormalizeFunc(utils.AliasFlags)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: stopCommand,
	})
	stopFlags(stopCommand.Flags())
	validate.AddLatestFlag(stopCommand, &stopOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerStopCommand,
		Parent:  containerCmd,
	})

	stopFlags(containerStopCommand.Flags())
	validate.AddLatestFlag(containerStopCommand, &stopOptions.Latest)
}

func stop(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if cmd.Flag("time").Changed {
		stopOptions.Timeout = &stopTimeout
	}

	responses, err := registry.ContainerEngine().ContainerStop(context.Background(), args, stopOptions)
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
