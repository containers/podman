package containers

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	rmDescription = `Removes one or more containers from the host. The container name or ID can be used.

  Command does not remove images. Running or unusable containers will not be removed without the -f option.`
	rmCommand = &cobra.Command{
		Use:   "rm [options] CONTAINER [CONTAINER...]",
		Short: "Remove one or more containers",
		Long:  rmDescription,
		RunE:  rm,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, true)
		},
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman rm imageID
  podman rm mywebserver myflaskserver 860a4b23
  podman rm --force --all
  podman rm -f c684f0d469f2`,
	}

	containerRmCommand = &cobra.Command{
		Use:               rmCommand.Use,
		Short:             rmCommand.Short,
		Long:              rmCommand.Long,
		RunE:              rmCommand.RunE,
		Args:              rmCommand.Args,
		ValidArgsFunction: rmCommand.ValidArgsFunction,
		Example: `podman container rm imageID
  podman container rm mywebserver myflaskserver 860a4b23
  podman container rm --force --all
  podman container rm -f c684f0d469f2`,
	}
)

var (
	rmOptions = entities.RmOptions{}
	cidFiles  = []string{}
)

func rmFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all containers")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified container is missing")
	flags.BoolVarP(&rmOptions.Force, "force", "f", false, "Force removal of a running or unusable container")
	flags.BoolVar(&rmOptions.Depend, "depend", false, "Remove container and all containers that depend on the selected container")
	timeFlagName := "time"
	flags.UintVarP(&stopTimeout, timeFlagName, "t", containerConfig.Engine.StopTimeout, "Seconds to wait for stop before killing the container")
	_ = cmd.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)
	flags.BoolVarP(&rmOptions.Volumes, "volumes", "v", false, "Remove anonymous volumes associated with the container")

	cidfileFlagName := "cidfile"
	flags.StringArrayVar(&cidFiles, cidfileFlagName, nil, "Read the container ID from the file")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

	if !registry.IsRemote() {
		// This option is deprecated, but needs to still exists for backwards compatibility
		flags.Bool("storage", false, "Remove container from storage library")
		_ = flags.MarkHidden("storage")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCommand,
	})
	rmFlags(rmCommand)
	validate.AddLatestFlag(rmCommand, &rmOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerRmCommand,
		Parent:  containerCmd,
	})
	rmFlags(containerRmCommand)
	validate.AddLatestFlag(containerRmCommand, &rmOptions.Latest)
}

func rm(cmd *cobra.Command, args []string) error {
	if cmd.Flag("time").Changed {
		if !rmOptions.Force {
			return errors.New("--force option must be specified to use the --time option")
		}
		rmOptions.Timeout = &stopTimeout
	}
	for _, cidFile := range cidFiles {
		content, err := ioutil.ReadFile(string(cidFile))
		if err != nil {
			return errors.Wrap(err, "error reading CIDFile")
		}
		id := strings.Split(string(content), "\n")[0]
		args = append(args, id)
	}

	if rmOptions.All {
		logrus.Debug("--all is set: enforcing --depend=true")
		rmOptions.Depend = true
	}

	return removeContainers(args, rmOptions, true)
}

// removeContainers will remove the specified containers (names or IDs).
// Allows for sharing removal logic across commands. If setExit is set,
// removeContainers will set the exit code according to the `podman-rm` man
// page.
func removeContainers(namesOrIDs []string, rmOptions entities.RmOptions, setExit bool) error {
	var (
		errs utils.OutputErrors
	)
	responses, err := registry.ContainerEngine().ContainerRm(context.Background(), namesOrIDs, rmOptions)
	if err != nil {
		if setExit {
			setExitCode(err)
		}
		return err
	}
	for _, r := range responses {
		if r.Err != nil {
			// TODO this will not work with the remote client
			if errors.Cause(err) == define.ErrWillDeadlock {
				logrus.Errorf("Potential deadlock detected - please run 'podman system renumber' to resolve")
			}
			if setExit {
				setExitCode(r.Err)
			}
			errs = append(errs, r.Err)
		} else {
			fmt.Println(r.Id)
		}
	}
	return errs.PrintErrors()
}

func setExitCode(err error) {
	// If error is set to no such container, do not reset
	if registry.GetExitCode() == 1 {
		return
	}
	cause := errors.Cause(err)
	switch {
	case cause == define.ErrNoSuchCtr:
		registry.SetExitCode(1)
	case strings.Contains(cause.Error(), define.ErrNoSuchCtr.Error()):
		registry.SetExitCode(1)
	case cause == define.ErrCtrStateInvalid:
		registry.SetExitCode(2)
	case strings.Contains(cause.Error(), define.ErrCtrStateInvalid.Error()):
		registry.SetExitCode(2)
	}
}
