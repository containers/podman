package server

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/coreos/go-systemd/activation"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type APIServer struct {
	http.Server        // The  HTTP work happens here
	*schema.Decoder    // Decoder for Query parameters to structs
	context.Context    // Context to carry objects to handlers
	*libpod.Runtime    // Where the real work happens
	net.Listener       // mux for routing HTTP API calls to libpod routines
	context.CancelFunc // Stop APIServer
	*time.Timer        // Hold timer for sliding window
	time.Duration      // Duration of client access sliding window
}

// Number of seconds to wait for next request, if exceeded shutdown server
const (
	DefaultServiceDuration   = 300 * time.Second
	UnlimitedServiceDuration = 0 * time.Second
)

// NewServer will create and configure a new API server with all defaults
func NewServer(runtime *libpod.Runtime) (*APIServer, error) {
	return newServer(runtime, DefaultServiceDuration, nil)
}

// NewServerWithSettings will create and configure a new API server using provided settings
func NewServerWithSettings(runtime *libpod.Runtime, duration time.Duration, listener *net.Listener) (*APIServer, error) {
	return newServer(runtime, duration, listener)
}

func newServer(runtime *libpod.Runtime, duration time.Duration, listener *net.Listener) (*APIServer, error) {
	// If listener not provided try socket activation protocol
	if listener == nil {
		if _, found := os.LookupEnv("LISTEN_FDS"); !found {
			return nil, errors.Errorf("Cannot create Server, no listener provided and socket activation protocol is not active.")
		}

		listeners, err := activation.Listeners()
		if err != nil {
			return nil, errors.Wrap(err, "Cannot retrieve file descriptors from systemd")
		}
		if len(listeners) != 1 {
			return nil, errors.Errorf("Wrong number of file descriptors for socket activation protocol (%d != 1)", len(listeners))
		}
		listener = &listeners[0]
	}

	router := mux.NewRouter()

	server := APIServer{
		Server: http.Server{
			Handler:           router,
			ReadHeaderTimeout: 20 * time.Second,
			ReadTimeout:       20 * time.Second,
			WriteTimeout:      2 * time.Minute,
		},
		Decoder:    schema.NewDecoder(),
		Context:    nil,
		Runtime:    runtime,
		Listener:   *listener,
		CancelFunc: nil,
		Duration:   duration,
	}
	server.Timer = time.AfterFunc(server.Duration, func() {
		if err := server.Shutdown(); err != nil {
			logrus.Errorf("unable to shutdown server: %q", err)
		}
	})

	ctx, cancelFn := context.WithCancel(context.Background())

	// TODO: Use ConnContext when ported to go 1.13
	ctx = context.WithValue(ctx, "decoder", server.Decoder)
	ctx = context.WithValue(ctx, "runtime", runtime)
	ctx = context.WithValue(ctx, "shutdownFunc", server.Shutdown)
	server.Context = ctx

	server.CancelFunc = cancelFn
	server.Decoder.IgnoreUnknownKeys(true)

	router.NotFoundHandler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// We can track user errors...
			logrus.Infof("Failed Request: (%d:%s) for %s:'%s'", http.StatusNotFound, http.StatusText(http.StatusNotFound), r.Method, r.URL.String())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		},
	)

	for _, fn := range []func(*mux.Router) error{
		server.RegisterAuthHandlers,
		server.RegisterContainersHandlers,
		server.RegisterDistributionHandlers,
		server.registerHealthCheckHandlers,
		server.registerImagesHandlers,
		server.registerInfoHandlers,
		server.RegisterMonitorHandlers,
		server.registerPingHandlers,
		server.RegisterPluginsHandlers,
		server.registerPodsHandlers,
		server.RegisterSwarmHandlers,
		server.registerSystemHandlers,
		server.registerVersionHandlers,
		server.registerVolumeHandlers,
	} {
		if err := fn(router); err != nil {
			return nil, err
		}
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		router.Walk(func(route *mux.Route, r *mux.Router, ancestors []*mux.Route) error { // nolint
			path, err := route.GetPathTemplate()
			if err != nil {
				path = ""
			}
			methods, err := route.GetMethods()
			if err != nil {
				methods = []string{}
			}
			logrus.Debugf("Methods: %s Path: %s", strings.Join(methods, ", "), path)
			return nil
		})
	}

	return &server, nil
}

// Serve starts responding to HTTP requests
func (s *APIServer) Serve() error {
	defer s.CancelFunc()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error, 1)

	go func() {
		err := s.Server.Serve(s.Listener)
		if err != nil && err != http.ErrServerClosed {
			errChan <- errors.Wrap(err, "Failed to start APIServer")
		}
		errChan <- nil
	}()

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		logrus.Infof("APIServer terminated by signal %v", sig)
	}

	return nil
}

// Shutdown is a clean shutdown waiting on existing clients
func (s *APIServer) Shutdown() error {
	// Duration == 0 flags no auto-shutdown of server
	if s.Duration == 0 {
		return nil
	}

	// We're still in the sliding service window
	if s.Timer.Stop() {
		s.Timer.Reset(s.Duration)
		return nil
	}

	// We've been idle for the service window, really shutdown
	go func() {
		err := s.Server.Shutdown(s.Context)
		if err != nil && err != context.Canceled {
			logrus.Errorf("Failed to cleanly shutdown APIServer: %s", err.Error())
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
