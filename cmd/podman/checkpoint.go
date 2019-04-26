package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	checkpointCommand     cliconfig.CheckpointValues
	checkpointDescription = `
   podman container checkpoint

   Checkpoints one or more running containers. The container name or ID can be used.
`
	_checkpointCommand = &cobra.Command{
		Use:   "checkpoint [flags] CONTAINER [CONTAINER...]",
		Short: "Checkpoints one or more containers",
		Long:  checkpointDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			checkpointCommand.InputArgs = args
			checkpointCommand.GlobalFlags = MainGlobalOpts
			checkpointCommand.Remote = remoteclient
			return checkpointCmd(&checkpointCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, false)
		},
		Example: `podman container checkpoint --keep ctrID
  podman container checkpoint --all
  podman container checkpoint --leave-running --latest`,
	}
)

func init() {
	checkpointCommand.Command = _checkpointCommand
	checkpointCommand.SetHelpTemplate(HelpTemplate())
	checkpointCommand.SetUsageTemplate(UsageTemplate())

	flags := checkpointCommand.Flags()
	flags.BoolVarP(&checkpointCommand.Keep, "keep", "k", false, "Keep all temporary checkpoint files")
	flags.BoolVarP(&checkpointCommand.LeaveRunning, "leave-running", "R", false, "Leave the container running after writing checkpoint to disk")
	flags.BoolVar(&checkpointCommand.TcpEstablished, "tcp-established", false, "Checkpoint a container with established TCP connections")
	flags.BoolVarP(&checkpointCommand.All, "all", "a", false, "Checkpoint all running containers")
	flags.BoolVarP(&checkpointCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	markFlagHiddenForRemoteClient("latest", flags)
}

func checkpointCmd(c *cliconfig.CheckpointValues) error {
	if rootless.IsRootless() {
		return errors.New("checkpointing a container requires root")
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	options := libpod.ContainerCheckpointOptions{
		Keep:           c.Keep,
		KeepRunning:    c.LeaveRunning,
		TCPEstablished: c.TcpEstablished,
	}
	return runtime.Checkpoint(c, options)
}
