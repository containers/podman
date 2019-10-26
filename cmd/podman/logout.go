package main

import (
	"fmt"

	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/pkg/errors"
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
	flags.BoolVarP(&logoutCommand.All, "all", "a", false, "Remove the cached credentials for all registries in the auth file")
	flags.StringVar(&logoutCommand.Authfile, "authfile", buildahcli.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	markFlagHiddenForRemoteClient("authfile", flags)
}

// logoutCmd uses the authentication package to remove the authenticated of a registry
// stored in the auth.json file
func logoutCmd(c *cliconfig.LogoutValues) error {
	args := c.InputArgs
	if len(args) > 1 {
		return errors.Errorf("too many arguments, logout takes at most 1 argument")
	}
	if len(args) == 0 && !c.All {
		return errors.Errorf("registry must be given")
	}
	var server string
	if len(args) == 1 {
		server = scrubServer(args[0])
	}

	sc, err := shared.GetSystemContext(c.Authfile)
	if err != nil {
		return err
	}

	if c.All {
		if err := config.RemoveAllAuthentication(sc); err != nil {
			return err
		}
		fmt.Println("Removed login credentials for all registries")
		return nil
	}

	err = config.RemoveAuthentication(sc, server)
	switch errors.Cause(err) {
	case nil:
		fmt.Printf("Removed login credentials for %s\n", server)
		return nil
	case config.ErrNotLoggedIn:
		// username of user logged in to server (if one exists)
		userFromAuthFile, passFromAuthFile, err := config.GetAuthentication(sc, server)
		if err != nil {
			return errors.Wrapf(err, "error reading auth file")
		}
		islogin := docker.CheckAuth(getContext(), sc, userFromAuthFile, passFromAuthFile, server)
		if userFromAuthFile != "" && passFromAuthFile != "" && islogin == nil {
			fmt.Printf("Not logged into %s with podman. Existing credentials were established via docker login. Please use docker logout instead.\n", server)
			return nil
		}
		fmt.Printf("Not logged into %s\n", server)
		return nil
	default:
		return errors.Wrapf(err, "error logging out of %q", server)
	}
}
