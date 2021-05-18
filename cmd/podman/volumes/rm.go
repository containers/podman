package volumes

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	volumeRmDescription = `Remove one or more existing volumes.

  By default only volumes that are not being used by any containers will be removed. To remove the volumes anyways, use the --force flag.`
	rmCommand = &cobra.Command{
		Use:               "rm [options] VOLUME [VOLUME...]",
		Aliases:           []string{"remove"},
		Short:             "Remove one or more volumes",
		Long:              volumeRmDescription,
		RunE:              rm,
		ValidArgsFunction: common.AutocompleteVolumes,
		Example: `podman volume rm myvol1 myvol2
  podman volume rm --all
  podman volume rm --force myvol`,
	}
)

var (
	rmOptions = entities.VolumeRmOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCommand,
		Parent:  volumeCmd,
	})
	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all volumes")
	flags.BoolVarP(&rmOptions.Force, "force", "f", false, "Remove a volume by force, even if it is being used by a container")
}

func rm(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if (len(args) > 0 && rmOptions.All) || (len(args) < 1 && !rmOptions.All) {
		return errors.New("choose either one or more volumes or all")
	}
	responses, err := registry.ContainerEngine().VolumeRm(context.Background(), args, rmOptions)
	if err != nil {
		setExitCode(err)
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			setExitCode(r.Err)
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func setExitCode(err error) {
	cause := errors.Cause(err)
	switch {
	case cause == define.ErrNoSuchVolume:
		registry.SetExitCode(1)
	case strings.Contains(cause.Error(), define.ErrNoSuchVolume.Error()):
		registry.SetExitCode(1)
	case cause == define.ErrVolumeBeingUsed:
		registry.SetExitCode(2)
	case strings.Contains(cause.Error(), define.ErrVolumeBeingUsed.Error()):
		registry.SetExitCode(2)
	}
}
