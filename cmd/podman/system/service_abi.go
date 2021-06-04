// +build linux,!remote

package system

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"

	api "github.com/containers/podman/v3/pkg/api/server"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra"
	"github.com/containers/podman/v3/pkg/util"
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
		path := opts.URI
		if fields[0] == "unix" {
			if path, err = filepath.Abs(fields[1]); err != nil {
				return err
			}
		}
		util.SetSocketPath(path)
		if os.Getenv("LISTEN_FDS") != "" {
			// If it is activated by systemd, use the first LISTEN_FD (3)
			// instead of opening the socket file.
			f := os.NewFile(uintptr(3), "podman.sock")
			l, err := net.FileListener(f)
			if err != nil {
				return err
			}
			listener = &l
		} else {
			network := fields[0]
			address := strings.Join(fields[1:], ":")
			l, err := net.Listen(network, address)
			if err != nil {
				return errors.Wrapf(err, "unable to create socket")
			}
			listener = &l
		}
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
	server, err := api.NewServerWithSettings(rt, listener, api.Options{Timeout: opts.Timeout, CorsHeaders: opts.CorsHeaders})
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
