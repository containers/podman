package serviceapi

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/coreos/go-systemd/activation"
)

type HttpServer struct {
	http.Server
	done     chan struct{}
	listener net.Listener
}

var libpodRuntime *libpod.Runtime

func NewServer(runtime *libpod.Runtime) (*HttpServer, error) {
	libpodRuntime = runtime

	listeners, err := activation.Listeners()
	if err != nil {
		return nil, errors.Wrap(err, "Cannot retrieve listeners")
	}
	if len(listeners) != 1 {
		return nil, errors.Wrapf(err, "unexpected number of socket activation (%d != 1)", len(listeners))
	}

	done := make(chan struct{})
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	server := HttpServer{http.Server{}, done, listeners[0]}
	go func() {
		<-quit
		log.Debugf("HttpServer is shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			log.Panicf("cannot gracefully shut down the http server: %s", err)
		}
		close(done)
	}()

	// TODO: build this into a map...
	http.Handle("/v1.24/images/json", serviceHandler(images))
	http.Handle("/v1.24/containers/json", serviceHandler(containers))
	return &server, nil
}

func (s *HttpServer) Serve() error {
	err := http.Serve(s.listener, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to start HttpServer")
	}
	<-s.done
	return nil
}

func (s *HttpServer) Shutdown(ctx context.Context) error {
	<-s.done
	return s.Server.Shutdown(ctx)
}

func (s *HttpServer) Close() error {
	return s.Server.Close()
}
