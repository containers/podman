package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/spf13/cobra"
)

var (
	restoreDescription = `
   podman container restore

   Restores a container from a checkpoint. The container name or ID can be used.
`
	restoreCommand = &cobra.Command{
		Use:   "restore [options] CONTAINER|IMAGE [CONTAINER|IMAGE...]",
		Short: "Restores one or more containers from a checkpoint",
		Long:  restoreDescription,
		RunE:  restore,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, true, "")
		},
		ValidArgsFunction: common.AutocompleteContainersAndImages,
		Example: `podman container restore ctrID
  podman container restore imageID
  podman container restore --latest
  podman container restore --all`,
	}
)

var restoreOptions entities.RestoreOptions

type restoreStatistics struct {
	PodmanDuration      int64                     `json:"podman_restore_duration"`
	ContainerStatistics []*entities.RestoreReport `json:"container_statistics"`
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: restoreCommand,
		Parent:  containerCmd,
	})
	flags := restoreCommand.Flags()
	flags.BoolVarP(&restoreOptions.All, "all", "a", false, "Restore all checkpointed containers")
	flags.BoolVarP(&restoreOptions.Keep, "keep", "k", false, "Keep all temporary checkpoint files")
	flags.BoolVar(&restoreOptions.TCPEstablished, "tcp-established", false, "Restore a container with established TCP connections")
	flags.BoolVar(&restoreOptions.FileLocks, "file-locks", false, "Restore a container with file locks")

	importFlagName := "import"
	flags.StringVarP(&restoreOptions.Import, importFlagName, "i", "", "Restore from exported checkpoint archive (tar.gz)")
	_ = restoreCommand.RegisterFlagCompletionFunc(importFlagName, completion.AutocompleteDefault)

	nameFlagName := "name"
	flags.StringVarP(&restoreOptions.Name, nameFlagName, "n", "", "Specify new name for container restored from exported checkpoint (only works with image or --import)")
	_ = restoreCommand.RegisterFlagCompletionFunc(nameFlagName, completion.AutocompleteNone)

	importPreviousFlagName := "import-previous"
	flags.StringVar(&restoreOptions.ImportPrevious, importPreviousFlagName, "", "Restore from exported pre-checkpoint archive (tar.gz)")
	_ = restoreCommand.RegisterFlagCompletionFunc(importPreviousFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&restoreOptions.IgnoreRootFS, "ignore-rootfs", false, "Do not apply root file-system changes when importing from exported checkpoint")
	flags.BoolVar(&restoreOptions.IgnoreStaticIP, "ignore-static-ip", false, "Ignore IP address set via --static-ip")
	flags.BoolVar(&restoreOptions.IgnoreStaticMAC, "ignore-static-mac", false, "Ignore MAC address set via --mac-address")
	flags.BoolVar(&restoreOptions.IgnoreVolumes, "ignore-volumes", false, "Do not export volumes associated with container")

	flags.StringSliceP(
		"publish", "p", []string{},
		"Publish a container's port, or a range of ports, to the host (default [])",
	)
	_ = restoreCommand.RegisterFlagCompletionFunc("publish", completion.AutocompleteNone)

	flags.StringVar(&restoreOptions.Pod, "pod", "", "Restore container into existing Pod (only works with image or --import)")
	_ = restoreCommand.RegisterFlagCompletionFunc("pod", common.AutocompletePodsRunning)

	flags.BoolVar(
		&restoreOptions.PrintStats,
		"print-stats",
		false,
		"Display restore statistics",
	)

	validate.AddLatestFlag(restoreCommand, &restoreOptions.Latest)
}

func restore(cmd *cobra.Command, args []string) error {
	var (
		e    error
		errs utils.OutputErrors
	)
	podmanStart := time.Now()
	if rootless.IsRootless() {
		return fmt.Errorf("restoring a container requires root")
	}

	// Check if the container exists (#15055)
	exists := &entities.BoolReport{Value: false}
	for _, ctr := range args {
		exists, e = registry.ContainerEngine().ContainerExists(registry.GetContext(), ctr, entities.ContainerExistsOptions{})
		if e != nil {
			return e
		}
		if exists.Value {
			break
		}
	}

	if !exists.Value {
		// Find out if this is an image
		inspectOpts := entities.InspectOptions{}
		imgData, _, err := registry.ImageEngine().Inspect(context.Background(), args, inspectOpts)
		if err != nil {
			return err
		}

		hostInfo, err := registry.ContainerEngine().Info(context.Background())
		if err != nil {
			return err
		}

		for i := range imgData {
			restoreOptions.CheckpointImage = true
			checkpointRuntimeName, found := imgData[i].Annotations[define.CheckpointAnnotationRuntimeName]
			if !found {
				return fmt.Errorf("image is not a checkpoint: %s", imgData[i].ID)
			}
			if hostInfo.Host.OCIRuntime.Name != checkpointRuntimeName {
				return fmt.Errorf("container image \"%s\" requires runtime: \"%s\"", imgData[i].ID, checkpointRuntimeName)
			}
		}
	}

	notImport := (!restoreOptions.CheckpointImage && restoreOptions.Import == "")

	if notImport && restoreOptions.ImportPrevious != "" {
		return fmt.Errorf("--import-previous can only be used with image or --import")
	}
	if notImport && restoreOptions.IgnoreRootFS {
		return fmt.Errorf("--ignore-rootfs can only be used with image or --import")
	}
	if notImport && restoreOptions.IgnoreVolumes {
		return fmt.Errorf("--ignore-volumes can only be used with image or --import")
	}
	if notImport && restoreOptions.Name != "" {
		return fmt.Errorf("--name can only be used with image or --import")
	}
	if notImport && restoreOptions.Pod != "" {
		return fmt.Errorf("--pod can only be used with image or --import")
	}
	if restoreOptions.Name != "" && restoreOptions.TCPEstablished {
		return fmt.Errorf("--tcp-established cannot be used with --name")
	}

	inputPorts, err := cmd.Flags().GetStringSlice("publish")
	if err != nil {
		return err
	}
	restoreOptions.PublishPorts = inputPorts

	argLen := len(args)
	if restoreOptions.Import != "" {
		if restoreOptions.All || restoreOptions.Latest {
			return fmt.Errorf("cannot use --import with --all or --latest")
		}
		if argLen > 0 {
			return fmt.Errorf("cannot use --import with positional arguments")
		}
	}
	if (restoreOptions.All || restoreOptions.Latest) && argLen > 0 {
		return fmt.Errorf("--all or --latest and containers cannot be used together")
	}
	if argLen < 1 && !restoreOptions.All && !restoreOptions.Latest && restoreOptions.Import == "" {
		return fmt.Errorf("you must provide at least one name or id")
	}
	if argLen > 1 && restoreOptions.Name != "" {
		return fmt.Errorf("--name can only be used with one checkpoint image")
	}
	responses, err := registry.ContainerEngine().ContainerRestore(context.Background(), args, restoreOptions)
	if err != nil {
		return err
	}
	podmanFinished := time.Now()

	var statistics restoreStatistics

	for _, r := range responses {
		if r.Err == nil {
			if restoreOptions.PrintStats {
				statistics.ContainerStatistics = append(statistics.ContainerStatistics, r)
			} else {
				fmt.Println(r.Id)
			}
		} else {
			errs = append(errs, r.Err)
		}
	}

	if restoreOptions.PrintStats {
		statistics.PodmanDuration = podmanFinished.Sub(podmanStart).Microseconds()
		j, err := json.MarshalIndent(statistics, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(j))
	}

	return errs.PrintErrors()
}
