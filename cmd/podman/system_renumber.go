package main

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	renumberCommand     cliconfig.SystemRenumberValues
	renumberDescription = `
        podman system renumber

        Migrate lock numbers to handle a change in maximum number of locks.
        Mandatory after the number of locks in libpod.conf is changed.
`

	_renumberCommand = &cobra.Command{
		Use:   "renumber",
		Args:  noSubArgs,
		Short: "Migrate lock numbers",
		Long:  renumberDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			renumberCommand.InputArgs = args
			renumberCommand.GlobalFlags = MainGlobalOpts
			return renumberCmd(&renumberCommand)
		},
	}
)

func init() {
	renumberCommand.Command = _renumberCommand
	renumberCommand.SetUsageTemplate(UsageTemplate())
}

func renumberCmd(c *cliconfig.SystemRenumberValues) error {
	// We need to pass one extra option to NewRuntime.
	// This will inform the OCI runtime to start a renumber.
	// That's controlled by the last argument to GetRuntime.
	r, err := libpodruntime.GetRuntimeRenumber(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error renumbering locks")
	}
	if err := r.Shutdown(false); err != nil {
		return err
	}

	return nil
}
