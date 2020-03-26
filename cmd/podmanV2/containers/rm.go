package containers

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/utils"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	rmDescription = `Removes one or more containers from the host. The container name or ID can be used.

  Command does not remove images. Running or unusable containers will not be removed without the -f option.`
	rmCommand = &cobra.Command{
		Use:               "rm [flags] CONTAINER [CONTAINER...]",
		Short:             "Remove one or more containers",
		Long:              rmDescription,
		RunE:              rm,
		PersistentPreRunE: preRunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, true)
		},
		Example: `podman rm imageID
  podman rm mywebserver myflaskserver 860a4b23
  podman rm --force --all
  podman rm -f c684f0d469f2`,
	}
)

var (
	rmOptions = entities.RmOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: rmCommand,
	})
	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all containers")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified container is missing")
	flags.BoolVarP(&rmOptions.Force, "force", "f", false, "Force removal of a running or unusable container.  The default is false")
	flags.BoolVarP(&rmOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&rmOptions.Storage, "storage", false, "Remove container from storage library")
	flags.BoolVarP(&rmOptions.Volumes, "volumes", "v", false, "Remove anonymous volumes associated with the container")
	flags.StringArrayVarP(&rmOptions.CIDFiles, "cidfile", "", nil, "Read the container ID from the file")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
		_ = flags.MarkHidden("ignore")
		_ = flags.MarkHidden("cidfile")
		_ = flags.MarkHidden("storage")
	}

}

func rm(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	// Storage conflicts with --all/--latest/--volumes/--cidfile/--ignore
	if rmOptions.Storage {
		if rmOptions.All || rmOptions.Ignore || rmOptions.Latest || rmOptions.Volumes || rmOptions.CIDFiles != nil {
			return errors.Errorf("--storage conflicts with --volumes, --all, --latest, --ignore and --cidfile")
		}
	}
	responses, err := registry.ContainerEngine().ContainerRm(context.Background(), args, rmOptions)
	if err != nil {
		// TODO exitcode is a global main variable to track exit codes.
		// we need this enabled
		//if len(c.InputArgs) < 2 {
		//	exitCode = setExitCode(err)
		//}
		return err
	}
	for _, r := range responses {
		if r.Err != nil {
			// TODO this will not work with the remote client
			if errors.Cause(err) == define.ErrWillDeadlock {
				logrus.Errorf("Potential deadlock detected - please run 'podman system renumber' to resolve")
			}
			errs = append(errs, r.Err)
		} else {
			fmt.Println(r.Id)
		}
	}
	return errs.PrintErrors()
}
