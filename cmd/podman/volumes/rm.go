package volumes

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
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
	rmOptions   = entities.VolumeRmOptions{}
	stopTimeout uint
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCommand,
		Parent:  volumeCmd,
	})
	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all volumes")
	flags.BoolVarP(&rmOptions.Force, "force", "f", false, "Remove a volume by force, even if it is being used by a container")
	timeFlagName := "time"
	flags.UintVarP(&stopTimeout, timeFlagName, "t", containerConfig.Engine.StopTimeout, "Seconds to wait for running containers to stop before killing the container")
	_ = rmCommand.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)
}

func rm(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if (len(args) > 0 && rmOptions.All) || (len(args) < 1 && !rmOptions.All) {
		return errors.New("choose either one or more volumes or all")
	}
	if cmd.Flag("time").Changed {
		if !rmOptions.Force {
			return errors.New("--force option must be specified to use the --time option")
		}
		rmOptions.Timeout = &stopTimeout
	}
	responses, err := registry.ContainerEngine().VolumeRm(context.Background(), args, rmOptions)
	if err != nil {
		setExitCode(rmOptions.Force, []error{err})
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	setExitCode(rmOptions.Force, errs)
	return errs.PrintErrors()
}

func setExitCode(force bool, errs []error) {
	var (
		// noSuchVolumeErrors indicates the requested volume does not exist
		noSuchVolumeErrors bool
		// inUseErrors indicates that a volume is being used by at least one container
		inUseErrors bool
	)

	if len(errs) == 0 {
		registry.SetExitCode(0)
	}

	for _, err := range errs {
		cause := errors.Cause(err)
		switch {
		case cause == define.ErrNoSuchVolume:
			noSuchVolumeErrors = true
		case strings.Contains(cause.Error(), define.ErrNoSuchVolume.Error()):
			noSuchVolumeErrors = true
		case cause == define.ErrVolumeBeingUsed:
			inUseErrors = true
		case strings.Contains(cause.Error(), define.ErrVolumeBeingUsed.Error()):
			inUseErrors = true

		}
	}

	switch {
	case inUseErrors:
		// being used by a container.
		registry.SetExitCode(2)
	case noSuchVolumeErrors && !inUseErrors:
		// One of the specified volumes did not exist, and no other
		if force {
			registry.SetExitCode(define.ExecErrorCodeIgnore)
		} else {
			registry.SetExitCode(1)
		}
	default:
		registry.SetExitCode(125)
	}
}
