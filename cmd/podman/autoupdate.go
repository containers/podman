package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	autoUpdateCommand     cliconfig.AutoUpdateValues
	autoUpdateDescription = `Auto update containers according to their auto-update policy.

Auto-update policies are specified with the "io.containers.autoupdate" label.`
	_autoUpdateCommand = &cobra.Command{
		Use:   "auto-update [flags]",
		Short: "Auto update containers according to their auto-update policy",
		Args:  noSubArgs,
		Long:  autoUpdateDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			restartCommand.InputArgs = args
			restartCommand.GlobalFlags = MainGlobalOpts
			return autoUpdateCmd(&restartCommand)
		},
		Example: `podman auto-update`,
	}
)

func init() {
	autoUpdateCommand.Command = _autoUpdateCommand
	autoUpdateCommand.SetHelpTemplate(HelpTemplate())
	autoUpdateCommand.SetUsageTemplate(UsageTemplate())
}

func autoUpdateCmd(c *cliconfig.RestartValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	units, failures := runtime.AutoUpdate()
	for _, unit := range units {
		fmt.Println(unit)
	}
	var finalErr error
	if len(failures) > 0 {
		finalErr = failures[0]
		for _, e := range failures[1:] {
			finalErr = errors.Errorf("%v\n%v", finalErr, e)
		}
	}
	return finalErr
}
