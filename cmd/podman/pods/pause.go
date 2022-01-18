package pods

import (
	"context"
	"fmt"

	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	podPauseDescription = `The pod name or ID can be used.

  All running containers within each specified pod will then be paused.`
	pauseCommand = &cobra.Command{
		Use:   "pause [options] POD [POD...]",
		Short: "Pause one or more pods",
		Long:  podPauseDescription,
		RunE:  pause,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		ValidArgsFunction: common.AutocompletePodsRunning,
		Example: `podman pod pause podID1 podID2
  podman pod pause --latest
  podman pod pause --all`,
	}
)

var (
	pauseOptions entities.PodPauseOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pauseCommand,
		Parent:  podCmd,
	})
	flags := pauseCommand.Flags()
	flags.BoolVarP(&pauseOptions.All, "all", "a", false, "Pause all running pods")
	validate.AddLatestFlag(pauseCommand, &pauseOptions.Latest)
}
func pause(_ *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	responses, err := registry.ContainerEngine().PodPause(context.Background(), args, pauseOptions)
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
