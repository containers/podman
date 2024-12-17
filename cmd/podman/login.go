package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
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
		Short:             "Log in to a container registry",
		Long:              "Log in to a container registry on a specified server.",
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
	secretFlagName := "secret"
	flags.BoolVar(&loginOptions.tlsVerify, "tls-verify", false, "Require HTTPS and verify certificates when contacting registries")
	flags.String(secretFlagName, "", "Retrieve password from a podman secret")
	_ = loginCommand.RegisterFlagCompletionFunc(secretFlagName, common.AutocompleteSecrets)

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

	secretName := cmd.Flag("secret").Value.String()
	if len(secretName) > 0 {
		if len(loginOptions.Password) > 0 {
			return errors.New("--secret can not be used with --password options")
		}
		if len(loginOptions.Username) == 0 {
			loginOptions.Username = secretName
		}
		var inspectOpts = entities.SecretInspectOptions{
			ShowSecret: true,
		}
		inspected, errs, _ := registry.ContainerEngine().SecretInspect(context.Background(), []string{secretName}, inspectOpts)

		if len(errs) > 0 && errs[0] != nil {
			return errs[0]
		}
		if len(inspected) == 0 {
			return fmt.Errorf("no secrets found for %q", secretName)
		}
		if len(inspected) > 1 {
			return fmt.Errorf("unexpected error SecretInspect of a single secret should never return more then one secrets %q", secretName)
		}
		loginOptions.Password = inspected[0].SecretData
	}

	sysCtx := &types.SystemContext{
		DockerInsecureSkipTLSVerify: skipTLS,
	}
	common.SetRegistriesConfPath(sysCtx)
	loginOptions.GetLoginSet = cmd.Flag("get-login").Changed
	return auth.Login(context.Background(), sysCtx, &loginOptions.LoginOptions, args)
}
