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
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultApiVersion = "1.40" // See https://docs.docker.com/engine/api/v1.40/
	MinimalApiVersion = "1.24"
)

type APIServer struct {
	http.Server        // Where the HTTP work happens
	*schema.Decoder    // Decoder for Query parameters to structs
	context.Context    // Context for graceful server shutdown
	*libpod.Runtime    // Where the real work happens
	net.Listener       // mux for routing HTTP API calls to libpod routines
	context.CancelFunc // Stop APIServer
	*time.Timer        // Hold timer for sliding window
	time.Duration      // Duration of client access sliding window
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
	signal.Notify(quit)

	ctx, cancel := context.WithCancel(context.Background())
	router := mux.NewRouter()

	server := APIServer{
		Server: http.Server{
			Handler:           router,
			ReadHeaderTimeout: 20 * time.Second,
			ReadTimeout:       20 * time.Second,
			WriteTimeout:      2 * time.Minute,
		},
		Decoder:    schema.NewDecoder(),
		Context:    ctx,
		Runtime:    runtime,
		Listener:   listeners[0],
		CancelFunc: cancel,
		Duration:   300 * time.Second,
	}
	server.Timer = time.AfterFunc(server.Duration, func() {
		server.Shutdown()
	})
	server.Decoder.IgnoreUnknownKeys(true)

	router.NotFoundHandler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Errorf("%d %s for %s:'%s'", http.StatusNotFound, http.StatusText(http.StatusNotFound), r.Method, r.URL.String())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		},
	)

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
	// We're still in the sliding service window
	if s.Timer.Stop() {
		s.Timer.Reset(s.Duration)
		return nil
	}

	// We've been idle for the service window, really shutdown
	go func() {
		err := s.Server.Shutdown(s.Context)
		if err != nil && err != context.Canceled {
			log.Errorf("Failed to cleanly shutdown APIServer: %s", err.Error())
		}
	}()

	// Wait for graceful shutdown vs. just killing connections and dropping data
	<-s.Context.Done()
	return nil
}

// Close immediately stops responding to clients and exits
func (s *APIServer) Close() error {
	return s.Server.Close()
}
