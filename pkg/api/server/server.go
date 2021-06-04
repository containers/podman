package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	goRuntime "runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/shutdown"
	"github.com/containers/podman/v3/pkg/api/handlers"
	"github.com/containers/podman/v3/pkg/api/server/idle"
	"github.com/coreos/go-systemd/v22/activation"
	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type APIServer struct {
	http.Server                      // The  HTTP work happens here
	*schema.Decoder                  // Decoder for Query parameters to structs
	context.Context                  // Context to carry objects to handlers
	*libpod.Runtime                  // Where the real work happens
	net.Listener                     // mux for routing HTTP API calls to libpod routines
	context.CancelFunc               // Stop APIServer
	idleTracker        *idle.Tracker // Track connections to support idle shutdown
	pprof              *http.Server  // Sidecar http server for providing performance data
	CorsHeaders        string        // Inject CORS headers to each request
}

// Number of seconds to wait for next request, if exceeded shutdown server
const (
	DefaultCorsHeaders       = ""
	DefaultServiceDuration   = 300 * time.Second
	UnlimitedServiceDuration = 0 * time.Second
)

// shutdownOnce ensures Shutdown() may safely be called from several go routines
var shutdownOnce sync.Once

type Options struct {
	Timeout     time.Duration
	CorsHeaders string
}

// NewServer will create and configure a new API server with all defaults
func NewServer(runtime *libpod.Runtime) (*APIServer, error) {
	return newServer(runtime, DefaultServiceDuration, nil, DefaultCorsHeaders)
}

// NewServerWithSettings will create and configure a new API server using provided settings
func NewServerWithSettings(runtime *libpod.Runtime, listener *net.Listener, opts Options) (*APIServer, error) {
	return newServer(runtime, opts.Timeout, listener, opts.CorsHeaders)
}

func newServer(runtime *libpod.Runtime, duration time.Duration, listener *net.Listener, corsHeaders string) (*APIServer, error) {
	// If listener not provided try socket activation protocol
	if listener == nil {
		if _, found := os.LookupEnv("LISTEN_PID"); !found {
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
	if corsHeaders == "" {
		logrus.Debug("CORS Headers were not set")
	} else {
		logrus.Debugf("CORS Headers were set to %s", corsHeaders)
	}

	logrus.Infof("API server listening on %q", (*listener).Addr())
	router := mux.NewRouter().UseEncodedPath()
	idle := idle.NewTracker(duration)

	server := APIServer{
		Server: http.Server{
			Handler:           router,
			ReadHeaderTimeout: 20 * time.Second,
			IdleTimeout:       duration * 2,
			ConnState:         idle.ConnState,
			ErrorLog:          log.New(logrus.StandardLogger().Out, "", 0),
		},
		Decoder:     handlers.NewAPIDecoder(),
		idleTracker: idle,
		Listener:    *listener,
		Runtime:     runtime,
		CorsHeaders: corsHeaders,
	}

	router.NotFoundHandler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// We can track user errors...
			logrus.Infof("Failed Request: (%d:%s) for %s:'%s'", http.StatusNotFound, http.StatusText(http.StatusNotFound), r.Method, r.URL.String())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		},
	)

	router.MethodNotAllowedHandler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// We can track user errors...
			logrus.Infof("Failed Request: (%d:%s) for %s:'%s'", http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), r.Method, r.URL.String())
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		},
	)

	for _, fn := range []func(*mux.Router) error{
		server.registerAuthHandlers,
		server.registerArchiveHandlers,
		server.registerContainersHandlers,
		server.registerDistributionHandlers,
		server.registerEventsHandlers,
		server.registerExecHandlers,
		server.registerGenerateHandlers,
		server.registerHealthCheckHandlers,
		server.registerImagesHandlers,
		server.registerInfoHandlers,
		server.registerManifestHandlers,
		server.registerMonitorHandlers,
		server.registerNetworkHandlers,
		server.registerPingHandlers,
		server.registerPlayHandlers,
		server.registerPluginsHandlers,
		server.registerPodsHandlers,
		server.registerSecretHandlers,
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

	if logrus.IsLevelEnabled(logrus.TraceLevel) {
		router.Walk(func(route *mux.Route, r *mux.Router, ancestors []*mux.Route) error { // nolint
			path, err := route.GetPathTemplate()
			if err != nil {
				path = "<N/A>"
			}
			methods, err := route.GetMethods()
			if err != nil {
				methods = []string{"<N/A>"}
			}
			logrus.Tracef("Methods: %6s Path: %s", strings.Join(methods, ", "), path)
			return nil
		})
	}

	return &server, nil
}

// If the NOTIFY_SOCKET is set, communicate the PID and readiness, and
// further unset NOTIFY_SOCKET to prevent containers from sending
// messages and unset INVOCATION_ID so conmon and containers are in
// the correct cgroup.
func setupSystemd() {
	if len(os.Getenv("NOTIFY_SOCKET")) == 0 {
		return
	}
	payload := fmt.Sprintf("MAINPID=%d\n", os.Getpid())
	payload += daemon.SdNotifyReady
	if sent, err := daemon.SdNotify(true, payload); err != nil {
		logrus.Errorf("Error notifying systemd of Conmon PID: %s", err.Error())
	} else if !sent {
		logrus.Warn("SDNotify not sent successfully")
	}

	if err := os.Unsetenv("INVOCATION_ID"); err != nil {
		logrus.Errorf("Error unsetting INVOCATION_ID: %s", err.Error())
	}
}

// Serve starts responding to HTTP requests.
func (s *APIServer) Serve() error {
	setupSystemd()

	if err := shutdown.Register("server", func(sig os.Signal) error {
		return s.Shutdown()
	}); err != nil {
		return err
	}
	// Start the shutdown signal handler.
	if err := shutdown.Start(); err != nil {
		return err
	}

	errChan := make(chan error, 1)

	go func() {
		<-s.idleTracker.Done()
		logrus.Debugf("API Server idle for %s", s.idleTracker.Duration.Round(time.Second).String())
		_ = s.Shutdown()
	}()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		go func() {
			pprofMux := mux.NewRouter()
			pprofMux.PathPrefix("/debug/pprof").Handler(http.DefaultServeMux)
			goRuntime.SetMutexProfileFraction(1)
			goRuntime.SetBlockProfileRate(1)
			s.pprof = &http.Server{Addr: "localhost:8888", Handler: pprofMux}
			err := s.pprof.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				logrus.Warn("Profiler Service failed: " + err.Error())
			}
		}()
	}

	// Before we start serving, ensure umask is properly set for container
	// creation.
	_ = syscall.Umask(0022)

	go func() {
		err := s.Server.Serve(s.Listener)
		if err != nil && err != http.ErrServerClosed {
			errChan <- errors.Wrap(err, "failed to start API server")
			return
		}
		errChan <- nil
	}()

	return <-errChan
}

// Shutdown is a clean shutdown waiting on existing clients
func (s *APIServer) Shutdown() error {
	if s.idleTracker.Duration == UnlimitedServiceDuration {
		logrus.Debug("APIServer.Shutdown ignored as Duration is UnlimitedService")
		return nil
	}

	shutdownOnce.Do(func() {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			_, file, line, _ := goRuntime.Caller(1)
			logrus.Debugf("APIServer.Shutdown by %s:%d, %d/%d connection(s)",
				file, line, s.idleTracker.ActiveConnections(), s.idleTracker.TotalConnections())

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), s.idleTracker.Duration)
				go func() {
					defer cancel()
					if err := s.pprof.Shutdown(ctx); err != nil {
						logrus.Warn(
							errors.Wrapf(err, "failed to cleanly shutdown pprof Server"))
					}
				}()
				<-ctx.Done()
			}()
		}

		// Gracefully shutdown server(s), duration of wait same as idle window
		ctx, cancel := context.WithTimeout(context.Background(), s.idleTracker.Duration)
		go func() {
			defer cancel()

			err := s.Server.Shutdown(ctx)
			if err != nil && err != context.Canceled && err != http.ErrServerClosed {
				logrus.Error(
					errors.Wrapf(err, "failed to cleanly shutdown APIServer"))
			}
		}()
		<-ctx.Done()
	})

	return nil
}

// Close immediately stops responding to clients and exits
func (s *APIServer) Close() error {
	return s.Server.Close()
}
