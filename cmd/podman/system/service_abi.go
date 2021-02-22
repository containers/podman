// +build linux,!remote

package system

import (
	"context"
	"net"
	"os"
	"strings"

	api "github.com/containers/podman/v3/pkg/api/server"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

func restService(opts entities.ServiceOptions, flags *pflag.FlagSet, cfg *entities.PodmanConfig) error {
	var (
		listener *net.Listener
		err      error
	)

	if opts.URI != "" {
		fields := strings.Split(opts.URI, ":")
		if len(fields) == 1 {
			return errors.Errorf("%s is an invalid socket destination", opts.URI)
		}
		address := strings.Join(fields[1:], ":")
		l, err := net.Listen(fields[0], address)
		if err != nil {
			return errors.Wrapf(err, "unable to create socket")
		}
		listener = &l
	}

	// Close stdin, so shortnames will not prompt
	devNullfile, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer devNullfile.Close()
	if err := unix.Dup2(int(devNullfile.Fd()), int(os.Stdin.Fd())); err != nil {
		return err
	}
	rt, err := infra.GetRuntime(context.Background(), flags, cfg)
	if err != nil {
		return err
	}

	infra.StartWatcher(rt)
	server, err := api.NewServerWithSettings(rt, opts.Timeout, listener)
	if err != nil {
		return err
	}
	defer func() {
		if err := server.Shutdown(); err != nil {
			logrus.Warnf("Error when stopping API service: %s", err)
		}
	}()

	err = server.Serve()
	if listener != nil {
		_ = (*listener).Close()
	}
	return err
}
