package network

import (
	"errors"
	"fmt"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networkrmDescription = `Remove networks`
	networkrmCommand     = &cobra.Command{
		Use:               "rm [options] NETWORK [NETWORK...]",
		Aliases:           []string{"remove"},
		Short:             "network rm",
		Long:              networkrmDescription,
		RunE:              networkRm,
		Example:           `podman network rm podman`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteNetworks,
	}
	stopTimeout uint
)

var (
	networkRmOptions entities.NetworkRmOptions
)

func networkRmFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&networkRmOptions.Force, "force", "f", false, "remove any containers using network")
	timeFlagName := "time"
	flags.UintVarP(&stopTimeout, timeFlagName, "t", containerConfig.Engine.StopTimeout, "Seconds to wait for running containers to stop before killing the container")
	_ = networkrmCommand.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkrmCommand,
		Parent:  networkCmd,
	})
	flags := networkrmCommand.Flags()
	networkRmFlags(flags)
}

func networkRm(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)

	if cmd.Flag("time").Changed {
		if !networkRmOptions.Force {
			return errors.New("--force option must be specified to use the --time option")
		}
		networkRmOptions.Timeout = &stopTimeout
	}
	responses, err := registry.ContainerEngine().NetworkRm(registry.Context(), args, networkRmOptions)
	if err != nil {
		setExitCode(networkRmOptions.Force, []error{err})
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Name)
		} else {
			errs = append(errs, r.Err)
		}
	}
	setExitCode(networkRmOptions.Force, errs)

	return errs.PrintErrors()
}

func setExitCode(force bool, errs []error) {
	var (
		// noSuchNetworkErrors indicates the requested network does not exist.
		noSuchNetworkErrors bool
		// inUseErrors indicates the requested operation failed because the network was in use
		inUseErrors bool
	)

	if len(errs) == 0 {
		registry.SetExitCode(0)
	}

	for _, err := range errs {
		if errors.Is(err, define.ErrNoSuchNetwork) || strings.Contains(err.Error(), define.ErrNoSuchNetwork.Error()) {
			noSuchNetworkErrors = true
		} else if errors.Is(err, define.ErrNetworkInUse) || strings.Contains(err.Error(), define.ErrNetworkInUse.Error()) {
			inUseErrors = true
		}
	}

	switch {
	case inUseErrors:
		// network is being used.
		registry.SetExitCode(2)
	case noSuchNetworkErrors && !inUseErrors:
		// One of the specified network did not exist, and no other
		// failures.
		if force {
			registry.SetExitCode(define.ExecErrorCodeIgnore)
		} else {
			registry.SetExitCode(1)
		}
	default:
		registry.SetExitCode(125)
	}
}
