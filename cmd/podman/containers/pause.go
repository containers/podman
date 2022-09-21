package containers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	pauseDescription = `Pauses one or more running containers.  The container name or ID can be used.`
	pauseCommand     = &cobra.Command{
		Use:   "pause [options] CONTAINER [CONTAINER...]",
		Short: "Pause all the processes in one or more containers",
		Long:  pauseDescription,
		RunE:  pause,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: common.AutocompleteContainersRunning,
		Example: `podman pause mywebserver
  podman pause 860a4b23
  podman pause --all`,
	}

	containerPauseCommand = &cobra.Command{
		Use:   pauseCommand.Use,
		Short: pauseCommand.Short,
		Long:  pauseCommand.Long,
		RunE:  pauseCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: pauseCommand.ValidArgsFunction,
		Example: `podman container pause mywebserver
  podman container pause 860a4b23
  podman container pause --all`,
	}
)

var (
	pauseOpts = entities.PauseUnPauseOptions{
		Filters: make(map[string][]string),
	}
	pauseCidFiles = []string{}
)

func pauseFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&pauseOpts.All, "all", "a", false, "Pause all running containers")

	cidfileFlagName := "cidfile"
	flags.StringArrayVar(&pauseCidFiles, cidfileFlagName, nil, "Read the container ID from the file")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

	filterFlagName := "filter"
	flags.StringSliceVarP(&filters, filterFlagName, "f", []string{}, "Filter output based on conditions given")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePsFilters)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cidfile")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pauseCommand,
	})
	pauseFlags(pauseCommand)
	validate.AddLatestFlag(pauseCommand, &pauseOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerPauseCommand,
		Parent:  containerCmd,
	})
	pauseFlags(containerPauseCommand)
	validate.AddLatestFlag(containerPauseCommand, &pauseOpts.Latest)
}

func pause(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)

	for _, cidFile := range pauseCidFiles {
		content, err := os.ReadFile(cidFile)
		if err != nil {
			return fmt.Errorf("reading CIDFile: %w", err)
		}
		id := strings.Split(string(content), "\n")[0]
		args = append(args, id)
	}

	for _, f := range filters {
		split := strings.SplitN(f, "=", 2)
		if len(split) < 2 {
			return fmt.Errorf("invalid filter %q", f)
		}
		pauseOpts.Filters[split[0]] = append(pauseOpts.Filters[split[0]], split[1])
	}

	responses, err := registry.ContainerEngine().ContainerPause(context.Background(), args, pauseOpts)
	if err != nil {
		return err
	}
	for _, r := range responses {
		switch {
		case r.Err != nil:
			errs = append(errs, r.Err)
		case r.RawInput != "":
			fmt.Println(r.RawInput)
		default:
			fmt.Println(r.Id)
		}
	}
	return errs.PrintErrors()
}
