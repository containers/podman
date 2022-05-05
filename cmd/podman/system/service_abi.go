//go:build linux && !remote
// +build linux,!remote

package system

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/containers/podman/v4/cmd/podman/registry"
	api "github.com/containers/podman/v4/pkg/api/server"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra"
	"github.com/containers/podman/v4/pkg/servicereaper"
	"github.com/coreos/go-systemd/v22/activation"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

func restService(flags *pflag.FlagSet, cfg *entities.PodmanConfig, opts entities.ServiceOptions) error {
	var (
		listener net.Listener
		err      error
	)

	libpodRuntime, err := infra.GetRuntime(registry.Context(), flags, cfg)
	if err != nil {
		return err
	}

	if opts.URI == "" {
		if _, found := os.LookupEnv("LISTEN_PID"); !found {
			return errors.New("no service URI provided and socket activation protocol is not active")
		}

		listeners, err := activation.Listeners()
		if err != nil {
			return fmt.Errorf("cannot retrieve file descriptors from systemd: %w", err)
		}
		if len(listeners) != 1 {
			return fmt.Errorf("wrong number of file descriptors for socket activation protocol (%d != 1)", len(listeners))
		}
		listener = listeners[0]
		libpodRuntime.SetRemoteURI(listeners[0].Addr().String())
	} else {
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
			if os.Getenv("LISTEN_FDS") != "" {
				// If it is activated by systemd, use the first LISTEN_FD (3)
				// instead of opening the socket file.
				f := os.NewFile(uintptr(3), "podman.sock")
				listener, err = net.FileListener(f)
				if err != nil {
					return err
				}
			} else {
				listener, err = net.Listen(uri.Scheme, path)
				if err != nil {
					return errors.Wrapf(err, "unable to create socket")
				}
			}
		case "tcp":
			host := uri.Host
			if host == "" {
				// For backward compatibility, support "tcp:<host>:<port>" and "tcp://<host>:<port>"
				host = uri.Opaque
			}
			listener, err = net.Listen(uri.Scheme, host)
			if err != nil {
				return errors.Wrapf(err, "unable to create socket %v", host)
			}
		default:
			logrus.Debugf("Attempting API Service endpoint scheme %q", uri.Scheme)
		}
		libpodRuntime.SetRemoteURI(uri.String())
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

	servicereaper.Start()
	infra.StartWatcher(libpodRuntime)
	server, err := api.NewServerWithSettings(libpodRuntime, listener, opts)
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
		_ = listener.Close()
	}
	return err
}
