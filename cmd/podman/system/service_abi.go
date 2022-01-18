// +build linux,!remote

package system

import (
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"

	api "github.com/containers/podman/v4/pkg/api/server"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra"
	"github.com/containers/podman/v4/pkg/servicereaper"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

func restService(flags *pflag.FlagSet, cfg *entities.PodmanConfig, opts entities.ServiceOptions) error {
	var (
		listener *net.Listener
		err      error
	)

	if opts.URI != "" {
		uri, err := url.Parse(opts.URI)
		if err != nil {
			return errors.Errorf("%s is an invalid socket destination", opts.URI)
		}

		switch uri.Scheme {
		case "unix":
			path, err := filepath.Abs(uri.Path)
			if err != nil {
				return err
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
				l, err := net.Listen(uri.Scheme, path)
				if err != nil {
					return errors.Wrapf(err, "unable to create socket")
				}
				listener = &l
			}
		case "tcp":
			host := uri.Host
			if host == "" {
				// For backward compatibility, support "tcp:<host>:<port>" and "tcp://<host>:<port>"
				host = uri.Opaque
			}
			l, err := net.Listen(uri.Scheme, host)
			if err != nil {
				return errors.Wrapf(err, "unable to create socket %v", host)
			}
			listener = &l
		default:
			logrus.Debugf("Attempting API Service endpoint scheme %q", uri.Scheme)
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

	servicereaper.Start()

	infra.StartWatcher(rt)
	server, err := api.NewServerWithSettings(rt, listener, opts)
	if err != nil {
		return err
	}
	defer func() {
		if err := server.Shutdown(true); err != nil {
			logrus.Warnf("Error when stopping API service: %s", err)
		}
	}()

	err = server.Serve()
	if listener != nil {
		_ = (*listener).Close()
	}
	return err
}
