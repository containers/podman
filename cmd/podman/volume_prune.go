package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	volumePruneCommand     cliconfig.VolumePruneValues
	volumePruneDescription = `
podman volume prune

Remove all unused volumes. Will prompt for confirmation if not
using force.
`
	_volumePruneCommand = &cobra.Command{
		Use:   "prune",
		Args:  noSubArgs,
		Short: "Remove all unused volumes",
		Long:  volumePruneDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			volumePruneCommand.InputArgs = args
			volumePruneCommand.GlobalFlags = MainGlobalOpts
			return volumePruneCmd(&volumePruneCommand)
		},
	}
)

func init() {
	volumePruneCommand.Command = _volumePruneCommand
	volumePruneCommand.SetUsageTemplate(UsageTemplate())
	flags := volumePruneCommand.Flags()

	flags.BoolVarP(&volumePruneCommand.Force, "force", "f", false, "Do not prompt for confirmation")
}

func volumePrune(runtime *adapter.LocalRuntime, ctx context.Context) error {
	prunedNames, prunedErrors := runtime.PruneVolumes(ctx)
	for _, name := range prunedNames {
		fmt.Println(name)
	}
	if len(prunedErrors) == 0 {
		return nil
	}
	// Grab the last error
	lastError := prunedErrors[len(prunedErrors)-1]
	// Remove the last error from the error slice
	prunedErrors = prunedErrors[:len(prunedErrors)-1]

	for _, err := range prunedErrors {
		logrus.Errorf("%q", err)
	}
	return lastError
}

func volumePruneCmd(c *cliconfig.VolumePruneValues) error {
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	// Prompt for confirmation if --force is not set
	if !c.Force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all volumes not used by at least one container.")
		fmt.Print("Are you sure you want to continue? [y/N] ")
		ans, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(err, "error reading input")
		}
		if strings.ToLower(ans)[0] != 'y' {
			return nil
		}
	}
	return volumePrune(runtime, getContext())
}
