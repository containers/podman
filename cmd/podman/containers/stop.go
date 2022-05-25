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
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	stopDescription = fmt.Sprintf(`Stops one or more running containers.  The container name or ID can be used.

  A timeout to forcibly stop the container can also be set but defaults to %d seconds otherwise.`, containerConfig.Engine.StopTimeout)
	stopCommand = &cobra.Command{
		Use:   "stop [options] CONTAINER [CONTAINER...]",
		Short: "Stop one or more containers",
		Long:  stopDescription,
		RunE:  stop,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: common.AutocompleteContainersRunning,
		Example: `podman stop ctrID
  podman stop --latest
  podman stop --time 2 mywebserver 6e534f14da9d`,
	}

	containerStopCommand = &cobra.Command{
		Use:   stopCommand.Use,
		Short: stopCommand.Short,
		Long:  stopCommand.Long,
		RunE:  stopCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: stopCommand.ValidArgsFunction,
		Example: `podman container stop ctrID
  podman container stop --latest
  podman container stop --time 2 mywebserver 6e534f14da9d`,
	}
)

var (
	stopOptions = entities.StopOptions{}
	stopTimeout uint
)

func stopFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&stopOptions.All, "all", "a", false, "Stop all running containers")
	flags.BoolVarP(&stopOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified container is missing")

	cidfileFlagName := "cidfile"
	flags.StringArrayVar(&cidFiles, cidfileFlagName, nil, "Read the container ID from the file")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

	timeFlagName := "time"
	flags.UintVarP(&stopTimeout, timeFlagName, "t", containerConfig.Engine.StopTimeout, "Seconds to wait for stop before killing the container")
	_ = cmd.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cidfile")
		_ = flags.MarkHidden("ignore")
	}

	flags.SetNormalizeFunc(utils.TimeoutAliasFlags)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: stopCommand,
	})
	stopFlags(stopCommand)
	validate.AddLatestFlag(stopCommand, &stopOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerStopCommand,
		Parent:  containerCmd,
	})
	stopFlags(containerStopCommand)
	validate.AddLatestFlag(containerStopCommand, &stopOptions.Latest)
}

func stop(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if cmd.Flag("time").Changed {
		stopOptions.Timeout = &stopTimeout
	}

	for _, cidFile := range cidFiles {
		content, err := ioutil.ReadFile(cidFile)
		if err != nil {
			return errors.Wrap(err, "error reading CIDFile")
		}
		id := strings.Split(string(content), "\n")[0]
		args = append(args, id)
	}

	responses, err := registry.ContainerEngine().ContainerStop(context.Background(), args, stopOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.RawInput)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}
