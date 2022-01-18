package containers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/criu"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage/pkg/archive"
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
		ValidArgsFunction: common.AutocompleteContainersRunning,
		Example: `podman container checkpoint --keep ctrID
  podman container checkpoint --all
  podman container checkpoint --leave-running --latest`,
	}
)

var checkpointOptions entities.CheckpointOptions

type checkpointStatistics struct {
	PodmanDuration      int64                        `json:"podman_checkpoint_duration"`
	ContainerStatistics []*entities.CheckpointReport `json:"container_statistics"`
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: checkpointCommand,
		Parent:  containerCmd,
	})
	flags := checkpointCommand.Flags()
	flags.BoolVarP(&checkpointOptions.Keep, "keep", "k", false, "Keep all temporary checkpoint files")
	flags.BoolVarP(&checkpointOptions.LeaveRunning, "leave-running", "R", false, "Leave the container running after writing checkpoint to disk")
	flags.BoolVar(&checkpointOptions.TCPEstablished, "tcp-established", false, "Checkpoint a container with established TCP connections")
	flags.BoolVar(&checkpointOptions.FileLocks, "file-locks", false, "Checkpoint a container with file locks")
	flags.BoolVarP(&checkpointOptions.All, "all", "a", false, "Checkpoint all running containers")

	exportFlagName := "export"
	flags.StringVarP(&checkpointOptions.Export, exportFlagName, "e", "", "Export the checkpoint image to a tar.gz")
	_ = checkpointCommand.RegisterFlagCompletionFunc(exportFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&checkpointOptions.IgnoreRootFS, "ignore-rootfs", false, "Do not include root file-system changes when exporting")
	flags.BoolVar(&checkpointOptions.IgnoreVolumes, "ignore-volumes", false, "Do not export volumes associated with container")
	flags.BoolVarP(&checkpointOptions.PreCheckPoint, "pre-checkpoint", "P", false, "Dump container's memory information only, leave the container running")
	flags.BoolVar(&checkpointOptions.WithPrevious, "with-previous", false, "Checkpoint container with pre-checkpoint images")

	flags.StringP("compress", "c", "zstd", "Select compression algorithm (gzip, none, zstd) for checkpoint archive.")
	_ = checkpointCommand.RegisterFlagCompletionFunc("compress", common.AutocompleteCheckpointCompressType)

	flags.BoolVar(
		&checkpointOptions.PrintStats,
		"print-stats",
		false,
		"Display checkpoint statistics",
	)

	validate.AddLatestFlag(checkpointCommand, &checkpointOptions.Latest)
}

func checkpoint(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	podmanStart := time.Now()
	if cmd.Flags().Changed("compress") {
		if checkpointOptions.Export == "" {
			return errors.Errorf("--compress can only be used with --export")
		}
		compress, _ := cmd.Flags().GetString("compress")
		switch strings.ToLower(compress) {
		case "none":
			checkpointOptions.Compression = archive.Uncompressed
		case "gzip":
			checkpointOptions.Compression = archive.Gzip
		case "zstd":
			checkpointOptions.Compression = archive.Zstd
		default:
			return errors.Errorf("Selected compression algorithm (%q) not supported. Please select one from: gzip, none, zstd", compress)
		}
	} else {
		checkpointOptions.Compression = archive.Zstd
	}
	if rootless.IsRootless() {
		return errors.New("checkpointing a container requires root")
	}
	if checkpointOptions.Export == "" && checkpointOptions.IgnoreRootFS {
		return errors.Errorf("--ignore-rootfs can only be used with --export")
	}
	if checkpointOptions.Export == "" && checkpointOptions.IgnoreVolumes {
		return errors.Errorf("--ignore-volumes can only be used with --export")
	}
	if checkpointOptions.WithPrevious && checkpointOptions.PreCheckPoint {
		return errors.Errorf("--with-previous can not be used with --pre-checkpoint")
	}
	if (checkpointOptions.WithPrevious || checkpointOptions.PreCheckPoint) && !criu.MemTrack() {
		return errors.New("system (architecture/kernel/CRIU) does not support memory tracking")
	}
	responses, err := registry.ContainerEngine().ContainerCheckpoint(context.Background(), args, checkpointOptions)
	if err != nil {
		return err
	}
	podmanFinished := time.Now()

	var statistics checkpointStatistics

	for _, r := range responses {
		if r.Err == nil {
			if checkpointOptions.PrintStats {
				statistics.ContainerStatistics = append(statistics.ContainerStatistics, r)
			} else {
				fmt.Println(r.Id)
			}
		} else {
			errs = append(errs, r.Err)
		}
	}

	if checkpointOptions.PrintStats {
		statistics.PodmanDuration = podmanFinished.Sub(podmanStart).Microseconds()
		j, err := json.MarshalIndent(statistics, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(j))
	}

	return errs.PrintErrors()
}
