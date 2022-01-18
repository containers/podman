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
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	restoreDescription = `
   podman container restore

   Restores a container from a checkpoint. The container name or ID can be used.
`
	restoreCommand = &cobra.Command{
		Use:   "restore [options] CONTAINER [CONTAINER...]",
		Short: "Restores one or more containers from a checkpoint",
		Long:  restoreDescription,
		RunE:  restore,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, true, false)
		},
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman container restore ctrID
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
	flags.StringVarP(&restoreOptions.Name, nameFlagName, "n", "", "Specify new name for container restored from exported checkpoint (only works with --import)")
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

	flags.StringVar(&restoreOptions.Pod, "pod", "", "Restore container into existing Pod (only works with --import)")
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
	var errs utils.OutputErrors
	podmanStart := time.Now()
	if rootless.IsRootless() {
		return errors.New("restoring a container requires root")
	}
	if restoreOptions.Import == "" && restoreOptions.ImportPrevious != "" {
		return errors.Errorf("--import-previous can only be used with --import")
	}
	if restoreOptions.Import == "" && restoreOptions.IgnoreRootFS {
		return errors.Errorf("--ignore-rootfs can only be used with --import")
	}
	if restoreOptions.Import == "" && restoreOptions.IgnoreVolumes {
		return errors.Errorf("--ignore-volumes can only be used with --import")
	}
	if restoreOptions.Import == "" && restoreOptions.Name != "" {
		return errors.Errorf("--name can only be used with --import")
	}
	if restoreOptions.Import == "" && restoreOptions.Pod != "" {
		return errors.Errorf("--pod can only be used with --import")
	}
	if restoreOptions.Name != "" && restoreOptions.TCPEstablished {
		return errors.Errorf("--tcp-established cannot be used with --name")
	}

	inputPorts, err := cmd.Flags().GetStringSlice("publish")
	if err != nil {
		return err
	}
	restoreOptions.PublishPorts = inputPorts

	argLen := len(args)
	if restoreOptions.Import != "" {
		if restoreOptions.All || restoreOptions.Latest {
			return errors.Errorf("Cannot use --import with --all or --latest")
		}
		if argLen > 0 {
			return errors.Errorf("Cannot use --import with positional arguments")
		}
	}
	if (restoreOptions.All || restoreOptions.Latest) && argLen > 0 {
		return errors.Errorf("--all or --latest and containers cannot be used together")
	}
	if argLen < 1 && !restoreOptions.All && !restoreOptions.Latest && restoreOptions.Import == "" {
		return errors.Errorf("you must provide at least one name or id")
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
