package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/shutdown"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/server/idle"
	"github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
)

type APIServer struct {
	http.Server                      // The  HTTP work happens here
	net.Listener                     // mux for routing HTTP API calls to libpod routines
	*libpod.Runtime                  // Where the real work happens
	*schema.Decoder                  // Decoder for Query parameters to structs
	context.CancelFunc               // Stop APIServer
	context.Context                  // Context to carry objects to handlers
	CorsHeaders        string        // Inject Cross-Origin Resource Sharing (CORS) headers
	PProfAddr          string        // Binding network address for pprof profiles
	idleTracker        *idle.Tracker // Track connections to support idle shutdown
}

// Number of seconds to wait for next request, if exceeded shutdown server
const (
	DefaultCorsHeaders       = ""
	DefaultServiceDuration   = 300 * time.Second
	UnlimitedServiceDuration = 0 * time.Second
)

var (
	// shutdownOnce ensures Shutdown() may safely be called from several go routines
	shutdownOnce sync.Once
)

// NewServer will create and configure a new API server with all defaults
func NewServer(runtime *libpod.Runtime) (*APIServer, error) {
	return newServer(runtime, nil, entities.ServiceOptions{
		CorsHeaders: DefaultCorsHeaders,
		Timeout:     DefaultServiceDuration,
	})
}

// NewServerWithSettings will create and configure a new API server using provided settings
func NewServerWithSettings(runtime *libpod.Runtime, listener net.Listener, opts entities.ServiceOptions) (*APIServer, error) {
	return newServer(runtime, listener, opts)
}

func newServer(runtime *libpod.Runtime, listener net.Listener, opts entities.ServiceOptions) (*APIServer, error) {
	logrus.Infof("API service listening on %q. URI: %q", listener.Addr(), runtime.RemoteURI())
	if opts.CorsHeaders == "" {
		logrus.Debug("CORS Headers were not set")
	} else {
		logrus.Debugf("CORS Headers were set to %q", opts.CorsHeaders)
	}

	logrus.Infof("API service listening on %q", listener.Addr())
	router := mux.NewRouter().UseEncodedPath()
	tracker := idle.NewTracker(opts.Timeout)

	server := APIServer{
		Server: http.Server{
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				return context.WithValue(ctx, types.ConnKey, c)
			},
			ConnState:   tracker.ConnState,
			ErrorLog:    log.New(logrus.StandardLogger().Out, "", 0),
			Handler:     router,
			IdleTimeout: opts.Timeout * 2,
		},
		CorsHeaders: opts.CorsHeaders,
		Listener:    listener,
		PProfAddr:   opts.PProfAddr,
		idleTracker: tracker,
	}

	server.BaseContext = func(l net.Listener) context.Context {
		ctx := context.WithValue(context.Background(), types.DecoderKey, handlers.NewAPIDecoder())
		ctx = context.WithValue(ctx, types.RuntimeKey, runtime)
		ctx = context.WithValue(ctx, types.IdleTrackerKey, tracker)
		return ctx
	}

	// Capture panics and print stack traces for diagnostics,
	// additionally process X-Reference-Id Header to support event correlation
	router.Use(panicHandler(), referenceIDHandler())
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
		server.registerSwaggerHandlers,
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
		// If in trace mode log request and response bodies
		router.Use(loggingHandler())
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

// setupSystemd notifies systemd API service is ready
// If the NOTIFY_SOCKET is set, communicate the PID and readiness, and unset INVOCATION_ID
// so conmon and containers are in the correct cgroup.
func (s *APIServer) setupSystemd() {
	if _, found := os.LookupEnv("NOTIFY_SOCKET"); !found {
		return
	}

	payload := fmt.Sprintf("MAINPID=%d\n", os.Getpid())
	payload += daemon.SdNotifyReady
	if sent, err := daemon.SdNotify(true, payload); err != nil {
		logrus.Error("API service failed to notify systemd of Conmon PID: " + err.Error())
	} else if !sent {
		logrus.Warn("API service unable to successfully send SDNotify")
	}

	if err := os.Unsetenv("INVOCATION_ID"); err != nil {
		logrus.Error("API service failed unsetting INVOCATION_ID: " + err.Error())
	}
}

// Serve starts responding to HTTP requests.
func (s *APIServer) Serve() error {
	s.setupPprof()

	if err := shutdown.Register("service", func(sig os.Signal) error {
		return s.Shutdown(true)
	}); err != nil {
		return err
	}
	// Start the shutdown signal handler.
	if err := shutdown.Start(); err != nil {
		return err
	}

	go func() {
		<-s.idleTracker.Done()
		logrus.Debugf("API service(s) shutting down, idle for %ds", int(s.idleTracker.Duration.Seconds()))
		_ = s.Shutdown(false)
	}()

	// Before we start serving, ensure umask is properly set for container creation.
	_ = syscall.Umask(0022)

	errChan := make(chan error, 1)
	s.setupSystemd()
	go func() {
		err := s.Server.Serve(s.Listener)
		if err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("failed to start API service: %w", err)
			return
		}
		errChan <- nil
	}()

	return <-errChan
}

// setupPprof enables pprof default endpoints
// Note: These endpoints and the podman flag --cpu-profile are mutually exclusive
//
// Examples:
// #1 go tool pprof -http localhost:8889 localhost:8888/debug/pprof/heap?seconds=120
// Note: web page will only render after a sample has been recorded
// #2 curl http://localhost:8888/debug/pprof/heap > heap.pprof && go tool pprof heap.pprof
func (s *APIServer) setupPprof() {
	if s.PProfAddr == "" {
		return
	}

	logrus.Infof("pprof service listening on %q", s.PProfAddr)
	go func() {
		old := runtime.SetMutexProfileFraction(1)
		defer runtime.SetMutexProfileFraction(old)

		runtime.SetBlockProfileRate(1)
		defer runtime.SetBlockProfileRate(0)

		router := mux.NewRouter()
		router.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)

		err := http.ListenAndServe(s.PProfAddr, router)
		if err != nil && err != http.ErrServerClosed {
			logrus.Warnf("pprof service failed: %v", err)
		}
	}()
}

// Shutdown is a clean shutdown waiting on existing clients
func (s *APIServer) Shutdown(halt bool) error {
	switch {
	case halt:
		logrus.Debug("API service forced shutdown, ignoring timeout Duration")
	case s.idleTracker.Duration == UnlimitedServiceDuration:
		logrus.Debug("API service shutdown request ignored as timeout Duration is UnlimitedService")
		return nil
	}

	shutdownOnce.Do(func() {
		logrus.Debugf("API service shutdown, %d/%d connection(s)",
			s.idleTracker.ActiveConnections(), s.idleTracker.TotalConnections())

		// Gracefully shutdown server(s), duration of wait same as idle window
		deadline := 1 * time.Second
		if s.idleTracker.Duration > 0 {
			deadline = s.idleTracker.Duration
		}
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		go func() {
			defer cancel()

			err := s.Server.Shutdown(ctx)
			if err != nil && err != context.Canceled && err != http.ErrServerClosed {
				logrus.Error("Failed to cleanly shutdown API service: " + err.Error())
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
