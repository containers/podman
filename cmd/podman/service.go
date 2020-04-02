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
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter"
	api "github.com/containers/libpod/pkg/api/server"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/systemd"
	"github.com/containers/libpod/pkg/util"
	iopodman "github.com/containers/libpod/pkg/varlink"
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
	flags.Int64VarP(&serviceCommand.Timeout, "timeout", "t", 5, "Time until the service session expires in seconds.  Use 0 to disable the timeout")
	flags.BoolVar(&serviceCommand.Varlink, "varlink", false, "Use legacy varlink service instead of REST")
}

func serviceCmd(c *cliconfig.ServiceValues) error {
	apiURI, err := resolveApiURI(c)
	if err != nil {
		return err
	}

	// Create a single runtime api consumption
	runtime, err := libpodruntime.GetRuntimeDisableFDs(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer func() {
		if err := runtime.Shutdown(false); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to shutdown libpod runtime: %v", err)
		}
	}()

	timeout := time.Duration(c.Timeout) * time.Second
	if c.Varlink {
		return runVarlink(runtime, apiURI, timeout, c)
	}
	return runREST(runtime, apiURI, timeout)
}

func resolveApiURI(c *cliconfig.ServiceValues) (string, error) {
	var apiURI string

	// When determining _*THE*_ listening endpoint --
	// 1) User input wins always
	// 2) systemd socket activation
	// 3) rootless honors XDG_RUNTIME_DIR
	// 4) if varlink -- adapter.DefaultVarlinkAddress
	// 5) lastly adapter.DefaultAPIAddress

	if len(c.InputArgs) > 0 {
		apiURI = c.InputArgs[0]
	} else if ok := systemd.SocketActivated(); ok { // nolint: gocritic
		apiURI = ""
	} else if rootless.IsRootless() {
		xdg, err := util.GetRuntimeDir()
		if err != nil {
			return "", err
		}
		socketName := "podman.sock"
		if c.Varlink {
			socketName = "io.podman"
		}
		socketDir := filepath.Join(xdg, "podman", socketName)
		if _, err := os.Stat(filepath.Dir(socketDir)); err != nil {
			if os.IsNotExist(err) {
				if err := os.Mkdir(filepath.Dir(socketDir), 0755); err != nil {
					return "", err
				}
			} else {
				return "", err
			}
		}
		apiURI = "unix:" + socketDir
	} else if c.Varlink {
		apiURI = adapter.DefaultVarlinkAddress
	} else {
		// For V2, default to the REST socket
		apiURI = adapter.DefaultAPIAddress
	}

	if "" == apiURI {
		logrus.Info("using systemd socket activation to determine API endpoint")
	} else {
		logrus.Infof("using API endpoint: %s", apiURI)
	}
	return apiURI, nil
}

func runREST(r *libpod.Runtime, uri string, timeout time.Duration) error {
	logrus.Warn("This function is EXPERIMENTAL")
	fmt.Println("This function is EXPERIMENTAL.")

	var listener *net.Listener
	if uri != "" {
		fields := strings.Split(uri, ":")
		if len(fields) == 1 {
			return errors.Errorf("%s is an invalid socket destination", uri)
		}
		address := strings.Join(fields[1:], ":")
		l, err := net.Listen(fields[0], address)
		if err != nil {
			return errors.Wrapf(err, "unable to create socket %s", uri)
		}
		listener = &l
	}
	server, err := api.NewServerWithSettings(r, timeout, listener)
	if err != nil {
		return err
	}
	defer func() {
		if err := server.Shutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "Error when stopping service: %s", err)
		}
	}()

	err = server.Serve()
	logrus.Debugf("%d/%d Active connections/Total connections\n", server.ActiveConnections, server.TotalConnections)
	return err
}

func runVarlink(r *libpod.Runtime, uri string, timeout time.Duration, c *cliconfig.ServiceValues) error {
	var varlinkInterfaces = []*iopodman.VarlinkInterface{varlinkapi.New(c.PodmanCommand.Command, r)}
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
			logrus.Infof("varlink service expired (use --timeout to increase session time beyond %s ms, 0 means never timeout)", timeout.String())
			return nil
		default:
			return errors.Wrapf(err, "unable to start varlink service")
		}
	}
	return nil
}
