package main

import (
	"context"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/spf13/cobra"
)

type loginOptionsWrapper struct {
	auth.LoginOptions
	tlsVerify bool
}

var (
	loginOptions = loginOptionsWrapper{}
	loginCommand = &cobra.Command{
		Use:               "login [options] [REGISTRY]",
		Short:             "Login to a container registry",
		Long:              "Login to a container registry on a specified server.",
		RunE:              login,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: common.AutocompleteRegistries,
		Example: `podman login quay.io
  podman login --username ... --password ... quay.io
  podman login --authfile dir/auth.json quay.io`,
	}
)

func init() {
	// Note that the local and the remote client behave the same: both
	// store credentials locally while the remote client will pass them
	// over the wire to the endpoint.
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: loginCommand,
	})
	flags := loginCommand.Flags()

	// Flags from the auth package.
	flags.AddFlagSet(auth.GetLoginFlags(&loginOptions.LoginOptions))

	// Add flag completion
	completion.CompleteCommandFlags(loginCommand, auth.GetLoginFlagsCompletions())

	// Podman flags.
	flags.BoolVarP(&loginOptions.tlsVerify, "tls-verify", "", false, "Require HTTPS and verify certificates when contacting registries")
	loginOptions.Stdin = os.Stdin
	loginOptions.Stdout = os.Stdout
	loginOptions.AcceptUnspecifiedRegistry = true
	loginOptions.AcceptRepositories = true
}

// Implementation of podman-login.
func login(cmd *cobra.Command, args []string) error {
	var skipTLS types.OptionalBool

	if cmd.Flags().Changed("tls-verify") {
		skipTLS = types.NewOptionalBool(!loginOptions.tlsVerify)
	}

	sysCtx := &types.SystemContext{
		AuthFilePath:                loginOptions.AuthFile,
		DockerCertPath:              loginOptions.CertDir,
		DockerInsecureSkipTLSVerify: skipTLS,
	}
	setRegistriesConfPath(sysCtx)
	loginOptions.GetLoginSet = cmd.Flag("get-login").Changed
	return auth.Login(context.Background(), sysCtx, &loginOptions.LoginOptions, args)
}

// setRegistriesConfPath sets the registries.conf path for the specified context.
// NOTE: this is a verbatim copy from c/common/libimage which we're not using
// to prevent leaking c/storage into this file.  Maybe this should go into c/image?
func setRegistriesConfPath(systemContext *types.SystemContext) {
	if systemContext.SystemRegistriesConfPath != "" {
		return
	}
	if envOverride, ok := os.LookupEnv("CONTAINERS_REGISTRIES_CONF"); ok {
		systemContext.SystemRegistriesConfPath = envOverride
		return
	}
	if envOverride, ok := os.LookupEnv("REGISTRIES_CONFIG_PATH"); ok {
		systemContext.SystemRegistriesConfPath = envOverride
		return
	}
}
