package containers

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

var (
	renameDescription = "The podman rename command allows you to rename an existing container or volume"
	renameCommand     = &cobra.Command{
		Use:               "rename {CONTAINER|VOLUME} NAME",
		Short:             "Rename an existing container or volume",
		Long:              renameDescription,
		RunE:              rename,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: autocompleteContainerOrVolumeOneArg,
		Example: `podman rename containerA newName
podman rename volumeA newName`,
	}

	containerRenameDescription = "The podman container rename command allows you to rename an existing container"
	containerRenameCommand     = &cobra.Command{
		Use:               "rename CONTAINER NAME",
		Short:             "Rename an existing container",
		Long:              containerRenameDescription,
		RunE:              containerRename,
		Args:              renameCommand.Args,
		ValidArgsFunction: common.AutocompleteContainerOneArg,
		Example:           "podman container rename containerA newName",
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: renameCommand,
	})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerRenameCommand,
		Parent:  containerCmd,
	})
}

func rename(_ *cobra.Command, args []string) error {
	if err := renameContainer(utils.RemoveSlash(args)); err == nil || !isNoSuch(err, define.ErrNoSuchCtr) {
		return err
	}

	if err := renameVolume(args); err == nil || !isNoSuch(err, define.ErrNoSuchVolume) {
		return err
	}

	return fmt.Errorf("no such object: %q", args[0])
}

func containerRename(_ *cobra.Command, args []string) error {
	args = utils.RemoveSlash(args)
	return renameContainer(args)
}

func renameContainer(args []string) error {
	renameOpts := entities.ContainerRenameOptions{
		NewName: args[1],
	}
	return registry.ContainerEngine().ContainerRename(registry.Context(), args[0], renameOpts)
}

func renameVolume(args []string) error {
	renameOpts := entities.VolumeRenameOptions{
		NewName: args[1],
	}
	return registry.ContainerEngine().VolumeRename(registry.Context(), args[0], renameOpts)
}

func isNoSuch(err error, target error) bool {
	return errors.Is(err, target) || errorhandling.Contains(err, target)
}

func autocompleteContainerOrVolumeOneArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	containers, _ := common.AutocompleteContainerOneArg(cmd, args, toComplete)
	volumes, _ := common.AutocompleteVolumes(cmd, args, toComplete)
	return append(containers, volumes...), cobra.ShellCompDirectiveNoFileComp
}
