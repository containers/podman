package containers

import (
	"errors"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	renameDescription = "The podman rename command allows you to rename an existing container"
	renameCommand     = &cobra.Command{
		Use:               "rename CONTAINER NAME",
		Short:             "Rename an existing container",
		Long:              renameDescription,
		RunE:              rename,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteContainerOneArg,
		Example:           "podman rename containerA newName",
	}

	containerRenameCommand = &cobra.Command{
		Use:               renameCommand.Use,
		Short:             renameCommand.Short,
		Long:              renameCommand.Long,
		RunE:              renameCommand.RunE,
		Args:              renameCommand.Args,
		ValidArgsFunction: renameCommand.ValidArgsFunction,
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

func rename(cmd *cobra.Command, args []string) error {
	if len(args) > 2 {
		return errors.New("must provide at least two arguments to rename")
	}
	args = utils.RemoveSlash(args)
	renameOpts := entities.ContainerRenameOptions{
		NewName: args[1],
	}
	return registry.ContainerEngine().ContainerRename(registry.GetContext(), args[0], renameOpts)
}
