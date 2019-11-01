// +build service,!remoteclient

package main

import (
	"context"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/pkg/serviceapi"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	serviceCommand     cliconfig.ServiceValues
	serviceDescription = `Run service interface.  Podman accepts a socket from systemd for incoming connections.

  Tools speaking http protocol can remotely manage pods, containers and images.
`
	_serviceCommand = &cobra.Command{
		Use:   "service [flags]",
		Short: "Run service interface",
		Long:  serviceDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceCommand.InputArgs = args
			serviceCommand.GlobalFlags = MainGlobalOpts
			return serviceCmd(&serviceCommand)
		},
		Example: `podman service
  podman service --timeout 5000`,
	}
)

func init() {
	serviceCommand.Command = _serviceCommand
	serviceCommand.SetHelpTemplate(HelpTemplate())
	serviceCommand.SetUsageTemplate(UsageTemplate())
	flags := serviceCommand.Flags()
	flags.Int64VarP(&serviceCommand.Timeout, "timeout", "t", 1000, "Time until the service session expires in milliseconds.  Use 0 to disable the timeout")
}

func serviceCmd(c *cliconfig.ServiceValues) error {
	if c.Timeout != 0 {
		var cancel context.CancelFunc
		_, cancel = context.WithTimeout(context.Background(), time.Duration(c.Timeout)*time.Millisecond)
		defer cancel()
	}

	// Create a single runtime for http
	runtime, err := libpodruntime.GetRuntimeDisableFDs(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.DeferredShutdown(false)

	server, _ := serviceapi.NewServer(runtime)
	_ = server.Serve()
	defer server.Shutdown()
	return nil
}
