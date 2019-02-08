// +build varlink

package main

import (
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/pkg/varlinkapi"
	"github.com/containers/libpod/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/varlink/go/varlink"
)

var (
	varlinkCommand     cliconfig.VarlinkValues
	varlinkDescription = `
	podman varlink

	run varlink interface
`
	_varlinkCommand = &cobra.Command{
		Use:   "varlink",
		Short: "Run varlink interface",
		Long:  varlinkDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			varlinkCommand.InputArgs = args
			varlinkCommand.GlobalFlags = MainGlobalOpts
			return varlinkCmd(&varlinkCommand)
		},
		Example: "VARLINK_URI",
	}
)

func init() {
	varlinkCommand.Command = _varlinkCommand
	flags := varlinkCommand.Flags()
	flags.Int64VarP(&varlinkCommand.Timeout, "timeout", "t", 1000, "Time until the varlink session expires in milliseconds.  Use 0 to disable the timeout")

	rootCmd.AddCommand(varlinkCommand.Command)
}

func varlinkCmd(c *cliconfig.VarlinkValues) error {
	args := c.InputArgs
	if len(args) < 1 {
		return errors.Errorf("you must provide a varlink URI")
	}
	timeout := time.Duration(c.Timeout) * time.Millisecond

	// Create a single runtime for varlink
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	var varlinkInterfaces = []*iopodman.VarlinkInterface{varlinkapi.New(&c.PodmanCommand, runtime)}
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
