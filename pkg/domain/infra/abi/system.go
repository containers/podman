// +build ABISupport

package abi

import (
	"context"
	"net"
	"strings"

	"github.com/containers/libpod/libpod/define"
	api "github.com/containers/libpod/pkg/api/server"
	"github.com/containers/libpod/pkg/domain/entities"
	iopodman "github.com/containers/libpod/pkg/varlink"
	iopodmanAPI "github.com/containers/libpod/pkg/varlinkapi"
	"github.com/containers/libpod/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/varlink/go/varlink"
)

func (ic *ContainerEngine) Info(ctx context.Context) (*define.Info, error) {
	return ic.Libpod.Info()
}

func (ic *ContainerEngine) RestService(_ context.Context, opts entities.ServiceOptions) error {
	var (
		listener net.Listener
		err      error
	)

	if opts.URI != "" {
		fields := strings.Split(opts.URI, ":")
		if len(fields) == 1 {
			return errors.Errorf("%s is an invalid socket destination", opts.URI)
		}
		address := strings.Join(fields[1:], ":")
		listener, err = net.Listen(fields[0], address)
		if err != nil {
			return errors.Wrapf(err, "unable to create socket %s", opts.URI)
		}
	}

	server, err := api.NewServerWithSettings(ic.Libpod, opts.Timeout, &listener)
	if err != nil {
		return err
	}
	defer func() {
		if err := server.Shutdown(); err != nil {
			logrus.Warnf("Error when stopping API service: %s", err)
		}
	}()

	err = server.Serve()
	logrus.Debugf("%d/%d Active connections/Total connections\n", server.ActiveConnections, server.TotalConnections)
	_ = listener.Close()
	return err
}

func (ic *ContainerEngine) VarlinkService(_ context.Context, opts entities.ServiceOptions) error {
	var varlinkInterfaces = []*iopodman.VarlinkInterface{
		iopodmanAPI.New(opts.Command, ic.Libpod),
	}

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
	if err = service.Listen(opts.URI, opts.Timeout); err != nil {
		switch err.(type) {
		case varlink.ServiceTimeoutError:
			logrus.Infof("varlink service expired (use --timeout to increase session time beyond %s ms, 0 means never timeout)", opts.Timeout.String())
			return nil
		default:
			return errors.Wrapf(err, "unable to start varlink service")
		}
	}
	return nil
}
