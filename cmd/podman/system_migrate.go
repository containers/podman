package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	migrateCommand     cliconfig.SystemMigrateValues
	migrateDescription = `
        podman system migrate

        Migrate existing containers to a new version of Podman.
`

	_migrateCommand = &cobra.Command{
		Use:   "migrate",
		Args:  noSubArgs,
		Short: "Migrate containers",
		Long:  migrateDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			migrateCommand.InputArgs = args
			migrateCommand.GlobalFlags = MainGlobalOpts
			return migrateCmd(&migrateCommand)
		},
	}
)

func init() {
	migrateCommand.Command = _migrateCommand
	migrateCommand.SetHelpTemplate(HelpTemplate())
	migrateCommand.SetUsageTemplate(UsageTemplate())
	flags := migrateCommand.Flags()
	flags.StringVar(&migrateCommand.NewRuntime, "new-runtime", "", "Specify a new runtime for all containers")
}

func migrateCmd(c *cliconfig.SystemMigrateValues) error {
	// We need to pass one extra option to NewRuntime.
	// This will inform the OCI runtime to start a migrate.
	// That's controlled by the last argument to GetRuntime.
	r, err := libpodruntime.GetRuntimeMigrate(getContext(), &c.PodmanCommand, c.NewRuntime)
	if err != nil {
		return errors.Wrapf(err, "error migrating containers")
	}
	if err := r.Shutdown(false); err != nil {
		return err
	}

	return nil
}
