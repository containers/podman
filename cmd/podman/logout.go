package main

import (
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/spf13/cobra"
)

var (
	logoutOptions = auth.LogoutOptions{}
	logoutCommand = &cobra.Command{
		Use:               "logout [options] [REGISTRY]",
		Short:             "Log out of a container registry",
		Long:              "Remove the cached username and password for the registry.",
		RunE:              logout,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: common.AutocompleteRegistries,
		Example: `podman logout quay.io
  podman logout --authfile dir/auth.json quay.io
  podman logout --all`,
	}
)

func init() {
	// Note that the local and the remote client behave the same: both
	// store credentials locally while the remote client will pass them
	// over the wire to the endpoint.
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: logoutCommand,
	})
	flags := logoutCommand.Flags()

	// Flags from the auth package.
	flags.AddFlagSet(auth.GetLogoutFlags(&logoutOptions))

	// Add flag completion
	completion.CompleteCommandFlags(logoutCommand, auth.GetLogoutFlagsCompletions())

	logoutOptions.Stdout = os.Stdout
	logoutOptions.AcceptUnspecifiedRegistry = true
	logoutOptions.AcceptRepositories = true
}

// Implementation of podman-logout.
func logout(cmd *cobra.Command, args []string) error {
	sysCtx := &types.SystemContext{}
	common.SetRegistriesConfPath(sysCtx)
	return auth.Logout(sysCtx, &logoutOptions, args)
}
