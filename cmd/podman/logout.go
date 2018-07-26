package main

import (
	"fmt"

	"github.com/containers/image/pkg/docker/config"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod/common"
	"github.com/urfave/cli"
)

var (
	logoutFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "authfile",
			Usage: "Path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
		},
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "Remove the cached credentials for all registries in the auth file",
		},
	}
	logoutDescription = "Remove the cached username and password for the registry."
	logoutCommand     = cli.Command{
		Name:        "logout",
		Usage:       "Logout of a container registry",
		Description: logoutDescription,
		Flags:       logoutFlags,
		Action:      logoutCmd,
		ArgsUsage:   "REGISTRY",
	}
)

// logoutCmd uses the authentication package to remove the authenticated of a registry
// stored in the auth.json file
func logoutCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) > 1 {
		return errors.Errorf("too many arguments, logout takes at most 1 argument")
	}
	if len(args) == 0 && !c.IsSet("all") {
		return errors.Errorf("registry must be given")
	}
	var server string
	if len(args) == 1 {
		server = args[0]
	}

	sc := common.GetSystemContext("", c.String("authfile"), false)

	if c.Bool("all") {
		if err := config.RemoveAllAuthentication(sc); err != nil {
			return err
		}
		fmt.Println("Remove login credentials for all registries")
		return nil
	}

	err := config.RemoveAuthentication(sc, server)
	switch err {
	case nil:
		fmt.Printf("Remove login credentials for %s\n", server)
		return nil
	case config.ErrNotLoggedIn:
		return errors.Errorf("Not logged into %s\n", server)
	default:
		return errors.Wrapf(err, "error logging out of %q", server)
	}
}
