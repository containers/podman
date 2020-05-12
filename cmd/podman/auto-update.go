package main

import (
	"fmt"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	autoUpdateOptions     = entities.AutoUpdateOptions{}
	autoUpdateDescription = `Auto update containers according to their auto-update policy.

  Auto-update policies are specified with the "io.containers.autoupdate" label.
  Note that this command is experimental. Please refer to the podman-auto-update(1) man page for details.`
	autoUpdateCommand = &cobra.Command{
		Use:   "auto-update [flags]",
		Short: "Auto update containers according to their auto-update policy",
		Long:  autoUpdateDescription,
		RunE:  autoUpdate,
		Example: `podman auto-update
  podman auto-update --authfile ~/authfile.json`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: autoUpdateCommand,
	})

	flags := autoUpdateCommand.Flags()
	flags.StringVar(&autoUpdateOptions.Authfile, "authfile", auth.GetDefaultAuthFile(), "Path to the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
}

func autoUpdate(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		// Backwards compat. System tests expext this error string.
		return errors.Errorf("`%s` takes no arguments", cmd.CommandPath())
	}
	report, failures := registry.ContainerEngine().AutoUpdate(registry.GetContext(), autoUpdateOptions)
	if report != nil {
		for _, unit := range report.Units {
			fmt.Println(unit)
		}
	}
	return errorhandling.JoinErrors(failures)
}
