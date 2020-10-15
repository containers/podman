package containers

import (
	"context"
	"fmt"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/utils"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	checkpointDescription = `
   podman container checkpoint

   Checkpoints one or more running containers. The container name or ID can be used.
`
	checkpointCommand = &cobra.Command{
		Use:   "checkpoint [options] CONTAINER [CONTAINER...]",
		Short: "Checkpoints one or more containers",
		Long:  checkpointDescription,
		RunE:  checkpoint,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		Example: `podman container checkpoint --keep ctrID
  podman container checkpoint --all
  podman container checkpoint --leave-running --latest`,
	}
)

var (
	checkpointOptions entities.CheckpointOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: checkpointCommand,
		Parent:  containerCmd,
	})
	flags := checkpointCommand.Flags()
	flags.BoolVarP(&checkpointOptions.Keep, "keep", "k", false, "Keep all temporary checkpoint files")
	flags.BoolVarP(&checkpointOptions.LeaveRunning, "leave-running", "R", false, "Leave the container running after writing checkpoint to disk")
	flags.BoolVar(&checkpointOptions.TCPEstablished, "tcp-established", false, "Checkpoint a container with established TCP connections")
	flags.BoolVarP(&checkpointOptions.All, "all", "a", false, "Checkpoint all running containers")
	flags.StringVarP(&checkpointOptions.Export, "export", "e", "", "Export the checkpoint image to a tar.gz")
	flags.BoolVar(&checkpointOptions.IgnoreRootFS, "ignore-rootfs", false, "Do not include root file-system changes when exporting")
	validate.AddLatestFlag(checkpointCommand, &checkpointOptions.Latest)
}

func checkpoint(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if rootless.IsRootless() {
		return errors.New("checkpointing a container requires root")
	}
	if checkpointOptions.Export == "" && checkpointOptions.IgnoreRootFS {
		return errors.Errorf("--ignore-rootfs can only be used with --export")
	}
	responses, err := registry.ContainerEngine().ContainerCheckpoint(context.Background(), args, checkpointOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}
