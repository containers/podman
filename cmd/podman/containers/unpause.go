package containers

import (
	"context"
	"errors"
	"fmt"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	unpauseDescription = `Unpauses one or more previously paused containers.  The container name or ID can be used.`
	unpauseCommand     = &cobra.Command{
		Use:               "unpause [options] CONTAINER [CONTAINER...]",
		Short:             "Unpause the processes in one or more containers",
		Long:              unpauseDescription,
		RunE:              unpause,
		ValidArgsFunction: common.AutocompleteContainersPaused,
		Example: `podman unpause ctrID
  podman unpause --all`,
	}
	unPauseOptions = entities.PauseUnPauseOptions{}

	containerUnpauseCommand = &cobra.Command{
		Use:               unpauseCommand.Use,
		Short:             unpauseCommand.Short,
		Long:              unpauseCommand.Long,
		RunE:              unpauseCommand.RunE,
		ValidArgsFunction: unpauseCommand.ValidArgsFunction,
		Example: `podman container unpause ctrID
  podman container unpause --all`,
	}
)

func unpauseFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&unPauseOptions.All, "all", "a", false, "Pause all running containers")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: unpauseCommand,
	})
	flags := unpauseCommand.Flags()
	unpauseFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerUnpauseCommand,
		Parent:  containerCmd,
	})

	unpauseCommandFlags := containerUnpauseCommand.Flags()
	unpauseFlags(unpauseCommandFlags)
}

func unpause(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if rootless.IsRootless() && !registry.IsRemote() {
		cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
		if !cgroupv2 {
			return errors.New("unpause is not supported for cgroupv1 rootless containers")
		}
	}
	if len(args) < 1 && !unPauseOptions.All {
		return errors.New("you must provide at least one container name or id")
	}
	responses, err := registry.ContainerEngine().ContainerUnpause(context.Background(), args, unPauseOptions)
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
