package main

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/pkg/registries"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	logoutCommand     cliconfig.LogoutValues
	logoutDescription = "Remove the cached username and password for the registry."
	_logoutCommand    = &cobra.Command{
		Use:   "logout [flags] REGISTRY",
		Short: "Logout of a container registry",
		Long:  logoutDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			logoutCommand.InputArgs = args
			logoutCommand.GlobalFlags = MainGlobalOpts
			logoutCommand.Remote = remoteclient
			logoutCommand.Stdin = os.Stdin
			logoutCommand.Stdout = os.Stdout
			return logoutCmd(&logoutCommand)
		},
		Example: `podman logout quay.io
  podman logout --all`,
	}
)

func init() {
	if !remote {
		_logoutCommand.Example = fmt.Sprintf("%s\n  podman logout --authfile authdir/myauths.json quay.io", _logoutCommand.Example)

	}
	logoutCommand.Command = _logoutCommand
	logoutCommand.SetHelpTemplate(HelpTemplate())
	logoutCommand.SetUsageTemplate(UsageTemplate())
	flags := logoutCommand.Flags()
	flags.AddFlagSet(auth.GetLogoutFlags(&logoutCommand.LogoutOptions))
	markFlagHiddenForRemoteClient("authfile", flags)
}

// logoutCmd uses the authentication package to remove the authenticated of a registry
// stored in the auth.json file
func logoutCmd(c *cliconfig.LogoutValues) error {
	args := c.InputArgs
	if len(args) > 1 {
		return errors.Errorf("too many arguments, logout takes at most 1 argument")
	}
	var server string
	if len(args) == 0 && !c.All {
		registriesFromFile, err := registries.GetRegistries()
		if err != nil || len(registriesFromFile) == 0 {
			return errors.Errorf("no registries found in registries.conf, a registry must be provided")
		}

		server = registriesFromFile[0]
		logrus.Debugf("registry not specified, default to the first registry %q from registries.conf", server)
	}
	if len(args) == 1 {
		server = args[0]
	}

	sc, err := shared.GetSystemContext(c.AuthFile)
	if err != nil {
		return err
	}
	return auth.Logout(sc, &c.LogoutOptions, server)
}
