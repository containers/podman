package main

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	restoreCommand     cliconfig.RestoreValues
	restoreDescription = `
   podman container restore

   Restores a container from a checkpoint. The container name or ID can be used.
`
	_restoreCommand = &cobra.Command{
		Use:   "restore",
		Short: "Restores one or more containers from a checkpoint",
		Long:  restoreDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			restoreCommand.InputArgs = args
			restoreCommand.GlobalFlags = MainGlobalOpts
			return restoreCmd(&restoreCommand)
		},
		Example: "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func init() {
	restoreCommand.Command = _restoreCommand
	flags := restoreCommand.Flags()
	flags.BoolVarP(&restoreCommand.All, "all", "a", false, "Restore all checkpointed containers")
	flags.BoolVarP(&restoreCommand.Keep, "keep", "k", false, "Keep all temporary checkpoint files")
	flags.BoolVarP(&restoreCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	// TODO: add ContainerStateCheckpointed
	flags.BoolVar(&restoreCommand.TcpEstablished, "tcp-established", false, "Checkpoint a container with established TCP connections")

	rootCmd.AddCommand(restoreCommand.Command)
}

func restoreCmd(c *cliconfig.RestoreValues) error {
	if rootless.IsRootless() {
		return errors.New("restoring a container requires root")
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	options := libpod.ContainerCheckpointOptions{
		Keep:           c.Keep,
		TCPEstablished: c.TcpEstablished,
	}

	if err := checkAllAndLatest(&c.PodmanCommand); err != nil {
		return err
	}

	containers, lastError := getAllOrLatestContainers(&c.PodmanCommand, runtime, libpod.ContainerStateExited, "checkpointed")

	for _, ctr := range containers {
		if err = ctr.Restore(context.TODO(), options); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to restore container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
