package containers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
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
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
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
	rmOptions = entities.RmOptions{
		Filters: make(map[string][]string),
	}
	rmCidFiles = []string{}
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
	flags.StringArrayVar(&rmCidFiles, cidfileFlagName, nil, "Read the container ID from the file")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

	filterFlagName := "filter"
	flags.StringSliceVar(&filters, filterFlagName, []string{}, "Filter output based on conditions given")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePsFilters)

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
	for _, cidFile := range rmCidFiles {
		content, err := os.ReadFile(cidFile)
		if err != nil {
			return fmt.Errorf("reading CIDFile: %w", err)
		}
		id := strings.Split(string(content), "\n")[0]
		args = append(args, id)
	}

	for _, f := range filters {
		split := strings.SplitN(f, "=", 2)
		if len(split) < 2 {
			return fmt.Errorf("invalid filter %q", f)
		}
		rmOptions.Filters[split[0]] = append(rmOptions.Filters[split[0]], split[1])
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
	var errs utils.OutputErrors
	responses, err := registry.ContainerEngine().ContainerRm(context.Background(), namesOrIDs, rmOptions)
	if err != nil {
		if rmOptions.Force && strings.Contains(err.Error(), define.ErrNoSuchCtr.Error()) {
			return nil
		}
		if setExit {
			setExitCode(err)
		}
		return err
	}
	for _, r := range responses {
		switch {
		case r.Err != nil:
			if errors.Is(r.Err, define.ErrWillDeadlock) {
				logrus.Errorf("Potential deadlock detected - please run 'podman system renumber' to resolve")
			}
			if rmOptions.Force && strings.Contains(r.Err.Error(), define.ErrNoSuchCtr.Error()) {
				continue
			}
			if setExit {
				setExitCode(r.Err)
			}
			errs = append(errs, r.Err)
		case r.RawInput != "":
			fmt.Println(r.RawInput)
		default:
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
	if errors.Is(err, define.ErrNoSuchCtr) || strings.Contains(err.Error(), define.ErrNoSuchCtr.Error()) {
		registry.SetExitCode(1)
	} else if errors.Is(err, define.ErrCtrStateInvalid) || strings.Contains(err.Error(), define.ErrCtrStateInvalid.Error()) {
		registry.SetExitCode(2)
	}
}
