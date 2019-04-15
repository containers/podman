package main

import (
	"context"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	prunePodsCommand     cliconfig.PrunePodsValues
	prunePodsDescription = `
	podman pod prune

	Removes all exited pods
`
	_prunePodsCommand = &cobra.Command{
		Use:   "prune",
		Args:  noSubArgs,
		Short: "Remove all stopped pods",
		Long:  prunePodsDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			prunePodsCommand.InputArgs = args
			prunePodsCommand.GlobalFlags = MainGlobalOpts
			return prunePodsCmd(&prunePodsCommand)
		},
	}
)

func init() {
	prunePodsCommand.Command = _prunePodsCommand
	prunePodsCommand.SetHelpTemplate(HelpTemplate())
	prunePodsCommand.SetUsageTemplate(UsageTemplate())
	flags := prunePodsCommand.Flags()
	flags.BoolVarP(&prunePodsCommand.Force, "force", "f", false, "Force removal of a running pods.  The default is false")
}

func prunePods(runtime *adapter.LocalRuntime, ctx context.Context, maxWorkers int, force bool) error {
	var deleteFuncs []shared.ParallelWorkerInput

	states := []string{shared.PodStateStopped, shared.PodStateExited}
	delPods, err := runtime.GetPodsByStatus(states)
	if err != nil {
		return err
	}
	if len(delPods) < 1 {
		return nil
	}
	for _, pod := range delPods {
		p := pod
		f := func() error {
			return runtime.RemovePod(ctx, p, force, force)
		}

		deleteFuncs = append(deleteFuncs, shared.ParallelWorkerInput{
			ContainerID:  p.ID(),
			ParallelFunc: f,
		})
	}
	// Run the parallel funcs
	deleteErrors, errCount := shared.ParallelExecuteWorkerPool(maxWorkers, deleteFuncs)
	return printParallelOutput(deleteErrors, errCount)
}

func prunePodsCmd(c *cliconfig.PrunePodsValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	maxWorkers := shared.Parallelize("rm")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	return prunePods(runtime, getContext(), maxWorkers, c.Bool("force"))
}
