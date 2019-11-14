// Package serviceapi provides a container compatible interface
package serviceapi

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/coreos/go-systemd/activation"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// See https://docs.docker.com/engine/api/v1.40/
const (
	DefaultApiVersion = "1.40"
	MinimalApiVersion = "1.24"
)

type APIServer struct {
	http.Server
	context.Context
	*libpod.Runtime
	net.Listener
	context.CancelFunc
}

// NewServer will create and configure a new API HTTP server
func NewServer(runtime *libpod.Runtime) (*APIServer, error) {
	listeners, err := activation.Listeners()
	if err != nil {
		return nil, errors.Wrap(err, "Cannot retrieve listeners")
	}
	if len(listeners) != 1 {
		return nil, errors.Wrapf(err, "unexpected number of socket activation (%d != 1)", len(listeners))
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	router := mux.NewRouter()

	server := APIServer{
		Server: http.Server{
			Handler:           router,
			ReadHeaderTimeout: 20 * time.Second,
			ReadTimeout:       20 * time.Second,
			WriteTimeout:      2 * time.Minute,
		},
		Context:    ctx,
		Runtime:    runtime,
		Listener:   listeners[0],
		CancelFunc: cancel,
	}

	for _, fn := range []func(*mux.Router) error{
		server.registerAuthHandlers,
		server.registerContainersHandlers,
		server.registerDistributionHandlers,
		server.registerImagesHandlers,
		server.registerInfoHandlers,
		server.registerMonitorHandlers,
		server.registerPingHandlers,
		server.registerPluginsHandlers,
		server.registerPodsHandlers,
		server.registerSwarmHandlers,
		server.registerSystemHandlers,
		server.registerVersionHandlers,
	} {
		fn(router)
	}
	server.registerNotFoundHandlers(router) // Should always be called last!

	if log.IsLevelEnabled(log.DebugLevel) {
		router.Walk(func(route *mux.Route, r *mux.Router, ancestors []*mux.Route) error {
			path, err := route.GetPathTemplate()
			if err != nil {
				path = ""
			}
			methods, err := route.GetMethods()
			if err != nil {
				methods = []string{}
			}
			log.Debugf("Methods: %s Path: %s", strings.Join(methods, ", "), path)
			return nil
		})
	}
	return &server, nil
}

// Serve starts responding to HTTP requests
func (s *APIServer) Serve() error {
	defer s.CancelFunc()

	err := s.Server.Serve(s.Listener)
	if err != nil && err != http.ErrServerClosed {
		return errors.Wrap(err, "Failed to start APIServer")
	}

	return nil
}

// Shutdown is a clean shutdown waiting on existing clients
func (s *APIServer) Shutdown() error {
	go func() {
		if err := s.Server.Shutdown(s.Context); err != nil {
			log.Errorf(err.Error())
		}
	}()

	<-s.Context.Done()
	return nil
}

// Close immediately stops responding to clients and exits
func (s *APIServer) Close() error {
	return s.Server.Close()
}
