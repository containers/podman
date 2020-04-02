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
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/coreos/go-systemd/v22/activation"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type APIServer struct {
	http.Server                 // The  HTTP work happens here
	*schema.Decoder             // Decoder for Query parameters to structs
	context.Context             // Context to carry objects to handlers
	*libpod.Runtime             // Where the real work happens
	net.Listener                // mux for routing HTTP API calls to libpod routines
	context.CancelFunc          // Stop APIServer
	*time.Timer                 // Hold timer for sliding window
	time.Duration               // Duration of client access sliding window
	ActiveConnections  uint64   // Number of handlers holding a connection
	TotalConnections   uint64   // Number of connections handled
	ConnectionCh       chan int // Channel for signalling handler enter/exit
}

// Number of seconds to wait for next request, if exceeded shutdown server
const (
	DefaultServiceDuration   = 300 * time.Second
	UnlimitedServiceDuration = 0 * time.Second
	EnterHandler             = 1
	ExitHandler              = -1
	NOOPHandler              = 0
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
			return nil, errors.Errorf("Cannot create API Server, no listener provided and socket activation protocol is not active.")
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

	router := mux.NewRouter().UseEncodedPath()
	server := APIServer{
		Server: http.Server{
			Handler:           router,
			ReadHeaderTimeout: 20 * time.Second,
			IdleTimeout:       duration,
		},
		Decoder:      handlers.NewAPIDecoder(),
		Runtime:      runtime,
		Listener:     *listener,
		Duration:     duration,
		ConnectionCh: make(chan int),
	}

	router.NotFoundHandler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// We can track user errors...
			logrus.Infof("Failed Request: (%d:%s) for %s:'%s'", http.StatusNotFound, http.StatusText(http.StatusNotFound), r.Method, r.URL.String())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		},
	)

	for _, fn := range []func(*mux.Router) error{
		server.registerAuthHandlers,
		server.registerContainersHandlers,
		server.registerDistributionHandlers,
		server.registerEventsHandlers,
		server.registerExecHandlers,
		server.registerHealthCheckHandlers,
		server.registerImagesHandlers,
		server.registerInfoHandlers,
		server.registerManifestHandlers,
		server.registerMonitorHandlers,
		server.registerPingHandlers,
		server.registerPluginsHandlers,
		server.registerPodsHandlers,
		server.RegisterSwaggerHandlers,
		server.registerSwarmHandlers,
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
	// This is initialized here as Timer is not needed until Serve'ing
	if s.Duration > 0 {
		s.Timer = time.AfterFunc(s.Duration, func() {
			s.ConnectionCh <- NOOPHandler
		})
		go s.ReadChannelWithTimeout()
	} else {
		go s.ReadChannelNoTimeout()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error, 1)

	go func() {
		err := s.Server.Serve(s.Listener)
		if err != nil && err != http.ErrServerClosed {
			errChan <- errors.Wrap(err, "Failed to start APIServer")
			return
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

func (s *APIServer) ReadChannelWithTimeout() {
	// stalker to count the connections.  Should the timer expire it will shutdown the service.
	for delta := range s.ConnectionCh {
		switch delta {
		case EnterHandler:
			s.Timer.Stop()
			s.ActiveConnections += 1
			s.TotalConnections += 1
		case ExitHandler:
			s.Timer.Stop()
			s.ActiveConnections -= 1
			if s.ActiveConnections == 0 {
				// Server will be shutdown iff the timer expires before being reset or stopped
				s.Timer = time.AfterFunc(s.Duration, func() {
					if err := s.Shutdown(); err != nil {
						logrus.Errorf("Failed to shutdown APIServer: %v", err)
						os.Exit(1)
					}
				})
			} else {
				s.Timer.Reset(s.Duration)
			}
		case NOOPHandler:
			// push the check out another duration...
			s.Timer.Reset(s.Duration)
		default:
			logrus.Warnf("ConnectionCh received unsupported input %d", delta)
		}
	}
}

func (s *APIServer) ReadChannelNoTimeout() {
	// stalker to count the connections.
	for delta := range s.ConnectionCh {
		switch delta {
		case EnterHandler:
			s.ActiveConnections += 1
			s.TotalConnections += 1
		case ExitHandler:
			s.ActiveConnections -= 1
		case NOOPHandler:
		default:
			logrus.Warnf("ConnectionCh received unsupported input %d", delta)
		}
	}
}

// Shutdown is a clean shutdown waiting on existing clients
func (s *APIServer) Shutdown() error {
	// Duration == 0 flags no auto-shutdown of the server
	if s.Duration == 0 {
		logrus.Debug("APIServer.Shutdown ignored as Duration == 0")
		return nil
	}
	logrus.Debugf("APIServer.Shutdown called %v, conn %d/%d", time.Now(), s.ActiveConnections, s.TotalConnections)

	// Gracefully shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), s.Duration)
	defer cancel()

	go func() {
		err := s.Server.Shutdown(ctx)
		if err != nil && err != context.Canceled && err != http.ErrServerClosed {
			logrus.Errorf("Failed to cleanly shutdown APIServer: %s", err.Error())
		}
	}()
	return nil
}

// Close immediately stops responding to clients and exits
func (s *APIServer) Close() error {
	return s.Server.Close()
}
