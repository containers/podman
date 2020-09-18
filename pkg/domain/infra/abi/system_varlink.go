// +build varlink

package abi

import (
	"context"

	"github.com/containers/podman/v2/pkg/domain/entities"
	iopodman "github.com/containers/podman/v2/pkg/varlink"
	iopodmanAPI "github.com/containers/podman/v2/pkg/varlinkapi"
	"github.com/containers/podman/v2/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/varlink/go/varlink"
)

func (ic *ContainerEngine) VarlinkService(_ context.Context, opts entities.ServiceOptions) error {
	var varlinkInterfaces = []*iopodman.VarlinkInterface{
		iopodmanAPI.New(opts.Command, ic.Libpod),
	}

	service, err := varlink.NewService(
		"Atomic",
		"podman",
		version.Version.String(),
		"https://github.com/containers/podman",
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
			logrus.Infof("varlink service expired (use --time to increase session time beyond %s ms, 0 means never timeout)", opts.Timeout.String())
			return nil
		default:
			return errors.Wrapf(err, "unable to start varlink service")
		}
	}
	return nil
}
