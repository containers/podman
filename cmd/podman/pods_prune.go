package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	podPruneCommand     cliconfig.PodPruneValues
	podPruneDescription = `
	podman pod prune

	Removes all exited pods
`
	_prunePodsCommand = &cobra.Command{
		Use:   "prune",
		Args:  noSubArgs,
		Short: "Remove all stopped pods and their containers",
		Long:  podPruneDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			podPruneCommand.InputArgs = args
			podPruneCommand.GlobalFlags = MainGlobalOpts
			return podPruneCmd(&podPruneCommand)
		},
	}
)

func init() {
	podPruneCommand.Command = _prunePodsCommand
	podPruneCommand.SetHelpTemplate(HelpTemplate())
	podPruneCommand.SetUsageTemplate(UsageTemplate())
	flags := podPruneCommand.Flags()
	flags.BoolVarP(&podPruneCommand.Force, "force", "f", false, "Force removal of all running pods.  The default is false")
}

func podPruneCmd(c *cliconfig.PodPruneValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	ok, failures, err := runtime.PrunePods(getContext(), c)
	if err != nil {
		return err
	}
	return printCmdResults(ok, failures)
}
