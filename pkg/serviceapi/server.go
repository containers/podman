package serviceapi

import (
	"context"
	"encoding/json"
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

func NewServer(runtime *libpod.Runtime) (*HttpServer, error) {
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
	http.HandleFunc("/v1.24/images/json", func(w http.ResponseWriter, r *http.Request) {
		images, err := runtime.ImageRuntime().GetImages()
		if err != nil {
			log.Panicf("Failed to get images: %s", err)
		}
		buffer, err := json.Marshal(images)
		if err != nil {
			log.Panicf("Failed to marshal images to json: %s", err)
		}
		io.WriteString(w, string(buffer))
	})
	return &HttpServer{server, done, listeners[0]}, nil
}

func (s *HttpServer) Serve() error {
	err := http.Serve(s.listener, nil)
	if err != nil {
		log.Panicf("Cannot start server: %s", err)
	}
	<- s.done
	return nil
}

func (s *HttpServer) Shutdown() error {
	<-s.done
	return nil
}

func (s *HttpServer) Close() error {
	return s.Close()
}
