// +build varlink,!remoteclient

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	api "github.com/containers/libpod/pkg/api/server"
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
	serviceCommand     cliconfig.ServiceValues
	serviceDescription = `Run an API service

Enable a listening service for API access to Podman commands.
`

	_serviceCommand = &cobra.Command{
		Use:   "service [flags] [URI]",
		Short: "Run API service",
		Long:  serviceDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceCommand.InputArgs = args
			serviceCommand.GlobalFlags = MainGlobalOpts
			return serviceCmd(&serviceCommand)
		},
	}
)

func init() {
	serviceCommand.Command = _serviceCommand
	serviceCommand.SetHelpTemplate(HelpTemplate())
	serviceCommand.SetUsageTemplate(UsageTemplate())
	flags := serviceCommand.Flags()
	flags.Int64VarP(&serviceCommand.Timeout, "timeout", "t", 1000, "Time until the service session expires in milliseconds.  Use 0 to disable the timeout")
	flags.BoolVar(&serviceCommand.Varlink, "varlink", false, "Use legacy varlink service instead of REST")
}

func serviceCmd(c *cliconfig.ServiceValues) error {
	// For V2, default to the REST socket
	apiURI := adapter.DefaultAPIAddress
	if c.Varlink {
		apiURI = adapter.DefaultVarlinkAddress
	}

	if rootless.IsRootless() {
		xdg, err := util.GetRuntimeDir()
		if err != nil {
			return err
		}
		socketName := "podman.sock"
		if c.Varlink {
			socketName = "io.podman"
		}
		socketDir := filepath.Join(xdg, "podman", socketName)
		if _, err := os.Stat(filepath.Dir(socketDir)); err != nil {
			if os.IsNotExist(err) {
				if err := os.Mkdir(filepath.Dir(socketDir), 0755); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		apiURI = fmt.Sprintf("unix:%s", socketDir)
	}

	if len(c.InputArgs) > 0 {
		apiURI = c.InputArgs[0]
	}

	logrus.Infof("using API endpoint: %s", apiURI)

	// Create a single runtime api consumption
	runtime, err := libpodruntime.GetRuntimeDisableFDs(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	timeout := time.Duration(c.Timeout) * time.Millisecond
	if c.Varlink {
		return runVarlink(runtime, apiURI, timeout, c)
	}
	return runREST(runtime, apiURI, timeout)
}

func runREST(r *libpod.Runtime, uri string, timeout time.Duration) error {
	logrus.Warn("This function is EXPERIMENTAL")
	fmt.Println("This function is EXPERIMENTAL.")
	fields := strings.Split(uri, ":")
	if len(fields) == 1 {
		return errors.Errorf("%s is an invalid socket destination", uri)
	}
	address := strings.Join(fields[1:], ":")
	l, err := net.Listen(fields[0], address)
	if err != nil {
		return errors.Wrapf(err, "unable to create socket %s", uri)
	}
	server, err := api.NewServerWithSettings(r, timeout, &l)
	if err != nil {
		return err
	}
	return server.Serve()
}

func runVarlink(r *libpod.Runtime, uri string, timeout time.Duration, c *cliconfig.ServiceValues) error {
	var varlinkInterfaces = []*iopodman.VarlinkInterface{varlinkapi.New(&c.PodmanCommand, r)}
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
	if err = service.Listen(uri, timeout); err != nil {
		switch err.(type) {
		case varlink.ServiceTimeoutError:
			logrus.Infof("varlink service expired (use --timeout to increase session time beyond %d ms, 0 means never timeout)", timeout.String())
			return nil
		default:
			return errors.Wrapf(err, "unable to start varlink service")
		}
	}
	return nil
}
