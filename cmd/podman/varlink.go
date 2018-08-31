// +build varlink

package main

import (
	"time"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/pkg/varlinkapi"
	"github.com/containers/libpod/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/varlink/go/varlink"
)

var (
	varlinkDescription = `
	podman varlink

	run varlink interface
`
	varlinkFlags = []cli.Flag{
		cli.IntFlag{
			Name:  "timeout, t",
			Usage: "time until the varlink session expires in milliseconds.  Use 0 to disable the timeout.",
			Value: 1000,
		},
	}
	varlinkCommand = &cli.Command{
		Name:         "varlink",
		Usage:        "Run varlink interface",
		Description:  varlinkDescription,
		Flags:        varlinkFlags,
		Action:       varlinkCmd,
		ArgsUsage:    "VARLINK_URI",
		OnUsageError: usageErrorHandler,
	}
)

func varlinkCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.Errorf("you must provide a varlink URI")
	}
	timeout := time.Duration(c.Int64("timeout")) * time.Millisecond

	// Create a single runtime for varlink
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	var varlinkInterfaces = []*iopodman.VarlinkInterface{varlinkapi.New(c, runtime)}
	// Register varlink service. The metadata can be retrieved with:
	// $ varlink info [varlink address URI]
	service, err := varlink.NewService(
		"Atomic",
		"podman",
		version.Version,
		"https://github.com/containers/libpod",
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
	if err = service.Listen(args[0], timeout); err != nil {
		switch err.(type) {
		case varlink.ServiceTimeoutError:
			logrus.Infof("varlink service expired (use --timeout to increase session time beyond %d ms, 0 means never timeout)", c.Int64("timeout"))
			return nil
		default:
			return errors.Errorf("unable to start varlink service")
		}
	}

	return nil
}
