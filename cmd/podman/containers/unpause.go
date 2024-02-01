package containers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/spf13/cobra"
)

var (
	unpauseDescription = `Unpauses one or more previously paused containers.  The container name or ID can be used.`
	unpauseCommand     = &cobra.Command{
		Use:   "unpause [options] CONTAINER [CONTAINER...]",
		Short: "Unpause the processes in one or more containers",
		Long:  unpauseDescription,
		RunE:  unpause,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: common.AutocompleteContainersPaused,
		Example: `podman unpause ctrID
  podman unpause --all`,
	}

	containerUnpauseCommand = &cobra.Command{
		Use:   unpauseCommand.Use,
		Short: unpauseCommand.Short,
		Long:  unpauseCommand.Long,
		RunE:  unpauseCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: unpauseCommand.ValidArgsFunction,
		Example: `podman container unpause ctrID
  podman container unpause --all`,
	}
)

var (
	unpauseOpts = entities.PauseUnPauseOptions{
		Filters: make(map[string][]string),
	}
	unpauseCidFiles = []string{}
)

func unpauseFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&unpauseOpts.All, "all", "a", false, "Unpause all paused containers")

	cidfileFlagName := "cidfile"
	flags.StringArrayVar(&unpauseCidFiles, cidfileFlagName, nil, "Read the container ID from the file")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

	filterFlagName := "filter"
	flags.StringArrayVarP(&filters, filterFlagName, "f", []string{}, "Filter output based on conditions given")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePsFilters)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cidfile")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: unpauseCommand,
	})
	unpauseFlags(unpauseCommand)
	validate.AddLatestFlag(unpauseCommand, &unpauseOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerUnpauseCommand,
		Parent:  containerCmd,
	})
	unpauseFlags(containerUnpauseCommand)
	validate.AddLatestFlag(containerUnpauseCommand, &unpauseOpts.Latest)
}

func unpause(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	args = utils.RemoveSlash(args)

	if rootless.IsRootless() && !registry.IsRemote() {
		cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
		if !cgroupv2 {
			return errors.New("unpause is not supported for cgroupv1 rootless containers")
		}
	}

	for _, cidFile := range unpauseCidFiles {
		content, err := os.ReadFile(cidFile)
		if err != nil {
			return fmt.Errorf("reading CIDFile: %w", err)
		}
		id := strings.Split(string(content), "\n")[0]
		args = append(args, id)
	}

	for _, f := range filters {
		fname, filter, hasFilter := strings.Cut(f, "=")
		if !hasFilter {
			return fmt.Errorf("invalid filter %q", f)
		}
		unpauseOpts.Filters[fname] = append(unpauseOpts.Filters[fname], filter)
	}

	responses, err := registry.ContainerEngine().ContainerUnpause(context.Background(), args, unpauseOpts)
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
