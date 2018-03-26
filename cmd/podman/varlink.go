package main

import (
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/ioprojectatomicpodman"
	"github.com/projectatomic/libpod/pkg/varlinkapi"
	"github.com/projectatomic/libpod/version"
	"github.com/urfave/cli"
	"github.com/varlink/go/varlink"
)

var (
	varlinkDescription = `
	podman varlink

	run varlink interface
`
	varlinkFlags   = []cli.Flag{}
	varlinkCommand = cli.Command{
		Name:        "varlink",
		Usage:       "Run varlink interface",
		Description: varlinkDescription,
		Flags:       varlinkFlags,
		Action:      varlinkCmd,
		ArgsUsage:   "VARLINK_URI",
	}
)

func varlinkCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.Errorf("you must provide a varlink URI")
	}

	var varlinkInterfaces = []*ioprojectatomicpodman.VarlinkInterface{varlinkapi.VarlinkLibpod}
	// Register varlink service. The metadata can be retrieved with:
	// $ varlink info [varlink address URI]
	service, err := varlink.NewService(
		"Atomic",
		"podman",
		version.Version,
		"https://github.com/projectatomic/libpod",
	)
	if err != nil {
		return errors.Wrapf(err, "unable to create new varlink service")
	}

	for _, i := range varlinkInterfaces {
		if err := service.RegisterInterface(i); err != nil {
			return errors.Errorf("unable to register varlink interface %v", i)
		}
	}

	// Run the varlink server at the given address
	if err = service.Listen(args[0], 0); err != nil {
		return errors.Errorf("unable to start varlink service")
	}

	return nil
}
