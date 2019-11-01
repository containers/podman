package serviceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/containers/libpod/libpod"
	log "github.com/sirupsen/logrus"

	"github.com/coreos/go-systemd/activation"
)

type HttpServer struct {
	*http.Server
	done     chan struct{}
	listener net.Listener
}

var libpodRuntime *libpod.Runtime

func NewServer(runtime *libpod.Runtime) (*HttpServer, error) {
	libpodRuntime = runtime

	listeners, err := activation.Listeners()
	if err != nil {
		log.Panicf("Cannot retrieve listeners: %s", err)
	}
	if len(listeners) != 1 {
		log.Panicf("unexpected number of socket activation (%d != 1)", len(listeners))
	}

	done := make(chan struct{})
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	server := &http.Server{}
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
	return &HttpServer{server, done, listeners[0]}, nil
}

func (s *HttpServer) Serve() error {
	err := http.Serve(s.listener, nil)
	if err != nil {
		log.Panicf("Cannot start server: %s", err)
	}
	<-s.done
	return nil
}

func (s *HttpServer) Shutdown() error {
	<-s.done
	return nil
}

func (s *HttpServer) Close() error {
	return s.Close()
}

type serviceHandler func(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime)

func (h serviceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r, libpodRuntime)
}

func images(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" {
		http.Error(w,
			fmt.Sprintf("%s is not a supported Content-Type", r.Header.Get("Content-Type")),
			http.StatusUnsupportedMediaType)
		return
	}

	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	buffer, err := json.Marshal(images)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, string(buffer))
}

func containers(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.NotFound(w, r)
}
