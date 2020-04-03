package containers

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	restoreDescription = `
   podman container restore

   Restores a container from a checkpoint. The container name or ID can be used.
`
	restoreCommand = &cobra.Command{
		Use:   "restore [flags] CONTAINER [CONTAINER...]",
		Short: "Restores one or more containers from a checkpoint",
		Long:  restoreDescription,
		RunE:  restore,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, true, false)
		},
		Example: `podman container restore ctrID
  podman container restore --latest
  podman container restore --all`,
	}
)

var (
	restoreOptions entities.RestoreOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: restoreCommand,
		Parent:  containerCmd,
	})
	flags := restoreCommand.Flags()
	flags.BoolVarP(&restoreOptions.All, "all", "a", false, "Restore all checkpointed containers")
	flags.BoolVarP(&restoreOptions.Keep, "keep", "k", false, "Keep all temporary checkpoint files")
	flags.BoolVarP(&restoreOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&restoreOptions.TCPEstablished, "tcp-established", false, "Restore a container with established TCP connections")
	flags.StringVarP(&restoreOptions.Import, "import", "i", "", "Restore from exported checkpoint archive (tar.gz)")
	flags.StringVarP(&restoreOptions.Name, "name", "n", "", "Specify new name for container restored from exported checkpoint (only works with --import)")
	flags.BoolVar(&restoreOptions.IgnoreRootFS, "ignore-rootfs", false, "Do not apply root file-system changes when importing from exported checkpoint")
	flags.BoolVar(&restoreOptions.IgnoreStaticIP, "ignore-static-ip", false, "Ignore IP address set via --static-ip")
	flags.BoolVar(&restoreOptions.IgnoreStaticMAC, "ignore-static-mac", false, "Ignore MAC address set via --mac-address")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func restore(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if rootless.IsRootless() {
		return errors.New("restoring a container requires root")
	}
	if restoreOptions.Import == "" && restoreOptions.IgnoreRootFS {
		return errors.Errorf("--ignore-rootfs can only be used with --import")
	}
	if restoreOptions.Import == "" && restoreOptions.Name != "" {
		return errors.Errorf("--name can only be used with --import")
	}
	if restoreOptions.Name != "" && restoreOptions.TCPEstablished {
		return errors.Errorf("--tcp-established cannot be used with --name")
	}

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
		return errors.Errorf("no arguments are needed with --all or --latest")
	}
	if argLen < 1 && !restoreOptions.All && !restoreOptions.Latest && restoreOptions.Import == "" {
		return errors.Errorf("you must provide at least one name or id")
	}
	responses, err := registry.ContainerEngine().ContainerRestore(context.Background(), args, restoreOptions)
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
