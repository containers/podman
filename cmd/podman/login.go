package main

import (
	"context"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/registries"
	"github.com/spf13/cobra"
)

type loginOptionsWrapper struct {
	auth.LoginOptions
	tlsVerify bool
}

var (
	loginOptions = loginOptionsWrapper{}
	loginCommand = &cobra.Command{
		Use:   "login [flags] [REGISTRY]",
		Short: "Login to a container registry",
		Long:  "Login to a container registry on a specified server.",
		RunE:  login,
		Args:  cobra.MaximumNArgs(1),
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
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: loginCommand,
	})
	flags := loginCommand.Flags()

	// Flags from the auth package.
	flags.AddFlagSet(auth.GetLoginFlags(&loginOptions.LoginOptions))

	// Podman flags.
	flags.BoolVarP(&loginOptions.tlsVerify, "tls-verify", "", false, "Require HTTPS and verify certificates when contacting registries")
	loginOptions.Stdin = os.Stdin
	loginOptions.Stdout = os.Stdout
	loginOptions.AcceptUnspecifiedRegistry = true
}

// Implementation of podman-login.
func login(cmd *cobra.Command, args []string) error {
	var skipTLS types.OptionalBool

	if cmd.Flags().Changed("tls-verify") {
		skipTLS = types.NewOptionalBool(!loginOptions.tlsVerify)
	}

	sysCtx := types.SystemContext{
		AuthFilePath:                loginOptions.AuthFile,
		DockerCertPath:              loginOptions.CertDir,
		DockerInsecureSkipTLSVerify: skipTLS,
		SystemRegistriesConfPath:    registries.SystemRegistriesConfPath(),
	}
	loginOptions.GetLoginSet = cmd.Flag("get-login").Changed
	return auth.Login(context.Background(), &sysCtx, &loginOptions.LoginOptions, args)
}
