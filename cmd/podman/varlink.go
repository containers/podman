// +build varlink,!remoteclient

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/pkg/varlinkapi"
	"github.com/containers/libpod/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/varlink/go/varlink"
)

var (
	varlinkCommand     cliconfig.VarlinkValues
	varlinkDescription = `Run varlink interface.  Podman varlink listens on the specified unix domain socket for incoming connects.

  Tools speaking varlink protocol can remotely manage pods, containers and images.
`
	_varlinkCommand = &cobra.Command{
		Use:   "varlink [flags] [URI]",
		Short: "Run varlink interface",
		Long:  varlinkDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			varlinkCommand.InputArgs = args
			varlinkCommand.GlobalFlags = MainGlobalOpts
			return varlinkCmd(&varlinkCommand)
		},
		Example: `podman varlink unix:/run/podman/io.podman
  podman varlink --timeout 5000 unix:/run/podman/io.podman`,
	}
)

func init() {
	varlinkCommand.Command = _varlinkCommand
	varlinkCommand.SetHelpTemplate(HelpTemplate())
	varlinkCommand.SetUsageTemplate(UsageTemplate())
	flags := varlinkCommand.Flags()
	flags.Int64VarP(&varlinkCommand.Timeout, "timeout", "t", 1000, "Time until the varlink session expires in milliseconds.  Use 0 to disable the timeout")
}

func varlinkCmd(c *cliconfig.VarlinkValues) error {
	varlinkURI := adapter.DefaultAddress
	if rootless.IsRootless() {
		xdg, err := util.GetRootlessRuntimeDir()
		if err != nil {
			return err
		}
		socketDir := filepath.Join(xdg, "podman/io.podman")
		if _, err := os.Stat(filepath.Dir(socketDir)); os.IsNotExist(err) {
			if err := os.Mkdir(filepath.Dir(socketDir), 0755); err != nil {
				return err
			}
		}
		varlinkURI = fmt.Sprintf("unix:%s", socketDir)
	}
	args := c.InputArgs

	if len(args) > 1 {
		return errors.Errorf("too many arguments. You may optionally provide 1")
	}

	if len(args) > 0 {
		varlinkURI = args[0]
	}

	logrus.Debugf("Using varlink socket: %s", varlinkURI)
	timeout := time.Duration(c.Timeout) * time.Millisecond

	// Create a single runtime for varlink
	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
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
	if err = service.Listen(varlinkURI, timeout); err != nil {
		switch err.(type) {
		case varlink.ServiceTimeoutError:
			logrus.Infof("varlink service expired (use --timeout to increase session time beyond %d ms, 0 means never timeout)", c.Int64("timeout"))
			return nil
		default:
			return errors.Wrapf(err, "unable to start varlink service")
		}
	}

	return nil
}
