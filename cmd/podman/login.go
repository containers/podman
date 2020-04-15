package main

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/registries"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	loginCommand cliconfig.LoginValues

	loginDescription = "Login to a container registry on a specified server."
	_loginCommand    = &cobra.Command{
		Use:   "login [flags] REGISTRY",
		Short: "Login to a container registry",
		Long:  loginDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			loginCommand.InputArgs = args
			loginCommand.GlobalFlags = MainGlobalOpts
			loginCommand.Remote = remoteclient
			loginCommand.Stdin = os.Stdin
			loginCommand.Stdout = os.Stdout
			return loginCmd(&loginCommand)
		},
		Example: `podman login -u testuser -p testpassword localhost:5000
  podman login -u testuser -p testpassword localhost:5000`,
	}
)

func init() {
	if !remote {
		_loginCommand.Example = fmt.Sprintf("%s\n  podman login --authfile authdir/myauths.json quay.io", _loginCommand.Example)

	}
	loginCommand.Command = _loginCommand
	loginCommand.SetHelpTemplate(HelpTemplate())
	loginCommand.SetUsageTemplate(UsageTemplate())
	flags := loginCommand.Flags()
	flags.AddFlagSet(auth.GetLoginFlags(&loginCommand.LoginOptions))
	flags.BoolVar(&loginCommand.GetLogin, "get-login", true, "Return the current login user for the registry")
	flags.BoolVar(&loginCommand.TlsVerify, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	// Disabled flags for the remote client
	markFlagHiddenForRemoteClient("authfile", flags)
	markFlagHiddenForRemoteClient("tls-verify", flags)
	markFlagHiddenForRemoteClient("cert-dir", flags)
}

// loginCmd uses the authentication package to store a user's authenticated credentials
// in an auth.json file for future use
func loginCmd(c *cliconfig.LoginValues) error {
	args := c.InputArgs
	if len(args) > 1 {
		return errors.Errorf("too many arguments, login takes only 1 argument")
	}
	var server string
	if len(args) == 0 {
		registriesFromFile, err := registries.GetRegistries()
		if err != nil || len(registriesFromFile) == 0 {
			return errors.Errorf("please specify a registry to login to")
		}

		server = registriesFromFile[0]
		logrus.Debugf("registry not specified, default to the first registry %q from registries.conf", server)

	} else {
		server = args[0]
	}

	if c.Flag("password").Changed {
		fmt.Fprintf(os.Stderr, "WARNING! Using --password via the cli is insecure. Please consider using --password-stdin\n")
	}

	sc := image.GetSystemContext("", c.AuthFile, false)
	if c.Flag("tls-verify").Changed {
		sc.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
	}
	if c.CertDir != "" {
		sc.DockerCertPath = c.CertDir
	}
	c.LoginOptions.GetLoginSet = c.Flag("get-login").Changed
	return auth.Login(getContext(), sc, &c.LoginOptions, server)
}
