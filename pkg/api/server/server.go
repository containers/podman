package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	goRuntime "runtime"
	"strings"
	"sync"
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
	http.Server                     // The  HTTP work happens here
	*schema.Decoder                 // Decoder for Query parameters to structs
	context.Context                 // Context to carry objects to handlers
	*libpod.Runtime                 // Where the real work happens
	net.Listener                    // mux for routing HTTP API calls to libpod routines
	context.CancelFunc              // Stop APIServer
	idleTracker        *IdleTracker // Track connections to support idle shutdown
	pprof              *http.Server // Sidecar http server for providing performance data
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

	router := mux.NewRouter().UseEncodedPath()
	idle := NewIdleTracker(duration)

	server := APIServer{
		Server: http.Server{
			Handler:           router,
			ReadHeaderTimeout: 20 * time.Second,
			IdleTimeout:       duration,
			ConnState:         idle.ConnState,
			ErrorLog:          log.New(logrus.StandardLogger().Out, "", 0),
		},
		Decoder:     handlers.NewAPIDecoder(),
		idleTracker: idle,
		Listener:    *listener,
		Runtime:     runtime,
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
				path = "<N/A>"
			}
			methods, err := route.GetMethods()
			if err != nil {
				methods = []string{"<N/A>"}
			}
			logrus.Debugf("Methods: %6s Path: %s", strings.Join(methods, ", "), path)
			return nil
		})
	}

	return &server, nil
}

// Serve starts responding to HTTP requests
func (s *APIServer) Serve() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error, 1)

	go func() {
		<-s.idleTracker.Done()
		logrus.Debugf("API Server idle for %v", s.idleTracker.Duration)
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

	go func() {
		err := s.Server.Serve(s.Listener)
		if err != nil && err != http.ErrServerClosed {
			errChan <- errors.Wrap(err, "failed to start API server")
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

// Shutdown is a clean shutdown waiting on existing clients
func (s *APIServer) Shutdown() error {
	if s.idleTracker.Duration == UnlimitedServiceDuration {
		logrus.Debug("APIServer.Shutdown ignored as Duration is UnlimitedService.")
		return nil
	}

	// Gracefully shutdown server(s), duration of wait same as idle window
	// TODO: Should we really wait the idle window for shutdown?
	ctx, cancel := context.WithTimeout(context.Background(), s.idleTracker.Duration)
	defer cancel()

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		_, file, line, _ := runtime.Caller(1)
		logrus.Debugf("APIServer.Shutdown by %s:%d, %d/%d connection(s)",
			file, line, s.idleTracker.ActiveConnections(), s.idleTracker.TotalConnections())
		if err := s.pprof.Shutdown(ctx); err != nil {
			logrus.Warn("Failed to cleanly shutdown pprof Server: " + err.Error())
		}
	}

	go func() {
		err := s.Server.Shutdown(ctx)
		if err != nil && err != context.Canceled && err != http.ErrServerClosed {
			logrus.Error("Failed to cleanly shutdown APIServer: " + err.Error())
		}
	}()
	<-ctx.Done()
	return nil
}

// Close immediately stops responding to clients and exits
func (s *APIServer) Close() error {
	return s.Server.Close()
}

type IdleTracker struct {
	active   map[net.Conn]struct{}
	total    int
	mux      sync.Mutex
	timer    *time.Timer
	Duration time.Duration
}

func NewIdleTracker(idle time.Duration) *IdleTracker {
	return &IdleTracker{
		active:   make(map[net.Conn]struct{}),
		Duration: idle,
		timer:    time.NewTimer(idle),
	}
}

func (t *IdleTracker) ConnState(conn net.Conn, state http.ConnState) {
	t.mux.Lock()
	defer t.mux.Unlock()

	oldActive := len(t.active)
	logrus.Debugf("IdleTracker %p:%v %d/%d connection(s)", conn, state, t.ActiveConnections(), t.TotalConnections())
	switch state {
	case http.StateNew, http.StateActive, http.StateHijacked:
		t.active[conn] = struct{}{}
		// stop the timer if we transitioned from idle
		if oldActive == 0 {
			t.timer.Stop()
		}
		t.total += 1
	case http.StateIdle, http.StateClosed:
		delete(t.active, conn)
		// Restart the timer if we've become idle
		if oldActive > 0 && len(t.active) == 0 {
			t.timer.Stop()
			t.timer.Reset(t.Duration)
		}
	}
}

func (t *IdleTracker) ActiveConnections() int {
	return len(t.active)
}

func (t *IdleTracker) TotalConnections() int {
	return t.total
}

func (t *IdleTracker) Done() <-chan time.Time {
	return t.timer.C
}
