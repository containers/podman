package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	pruneContainersCommand     cliconfig.PruneContainersValues
	pruneContainersDescription = `
	podman container prune

	Removes all exited containers
`
	_pruneContainersCommand = &cobra.Command{
		Use:   "prune",
		Args:  noSubArgs,
		Short: "Remove all stopped containers",
		Long:  pruneContainersDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pruneContainersCommand.InputArgs = args
			pruneContainersCommand.GlobalFlags = MainGlobalOpts
			pruneContainersCommand.Remote = remoteclient
			return pruneContainersCmd(&pruneContainersCommand)
		},
	}
)

func init() {
	pruneContainersCommand.Command = _pruneContainersCommand
	pruneContainersCommand.SetHelpTemplate(HelpTemplate())
	pruneContainersCommand.SetUsageTemplate(UsageTemplate())
	flags := pruneContainersCommand.Flags()
	flags.BoolVarP(&pruneContainersCommand.Force, "force", "f", false, "Force removal of a running container.  The default is false")
}

func pruneContainersCmd(c *cliconfig.PruneContainersValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	maxWorkers := shared.DefaultPoolSize("prune")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	ok, failures, err := runtime.Prune(getContext(), maxWorkers, c.Force)
	if err != nil {
		if errors.Cause(err) == libpod.ErrNoSuchCtr {
			if len(c.InputArgs) > 1 {
				exitCode = 125
			} else {
				exitCode = 1
			}
		}
		return err
	}
	if len(failures) > 0 {
		exitCode = 125
	}
	return printCmdResults(ok, failures)
}
