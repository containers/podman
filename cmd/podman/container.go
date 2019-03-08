package main

import (
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
)

var (
	containerDescription = "Manage containers"
	containerCommand     = cliconfig.PodmanCommand{
		Command: &cobra.Command{
			Use:              "container",
			Short:            "Manage Containers",
			Long:             containerDescription,
			TraverseChildren: true,
			RunE:             commandRunE(),
		},
	}

	listSubCommand  cliconfig.PsValues
	_listSubCommand = &cobra.Command{
		Use:     strings.Replace(_psCommand.Use, "ps", "list", 1),
		Args:    noSubArgs,
		Short:   _psCommand.Short,
		Long:    _psCommand.Long,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			listSubCommand.InputArgs = args
			listSubCommand.GlobalFlags = MainGlobalOpts
			return psCmd(&listSubCommand)
		},
		Example: strings.Replace(_psCommand.Example, "podman ps", "podman container list", -1),
	}

	// Commands that are universally implemented.
	containerCommands = []*cobra.Command{
		_containerExistsCommand,
		_inspectCommand,
		_listSubCommand,
	}
)

func init() {
	listSubCommand.Command = _listSubCommand
	psInit(&listSubCommand)

	containerCommand.AddCommand(containerCommands...)
	containerCommand.AddCommand(getContainerSubCommands()...)
	containerCommand.SetUsageTemplate(UsageTemplate())

	rootCmd.AddCommand(containerCommand.Command)
}
