package shutdown

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	stopped         bool
	sigChan         chan os.Signal
	cancelChan      chan bool
	handlers        map[string]func() error
	shutdownInhibit sync.RWMutex
)

// Start begins handling SIGTERM and SIGINT and will run the given on-signal
// handlers when one is called. This can be cancelled by calling Stop().
func Start() error {
	if sigChan != nil && !stopped {
		// Already running, do nothing.
		return nil
	}

	sigChan = make(chan os.Signal, 1)
	cancelChan = make(chan bool, 1)
	stopped = false

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-cancelChan:
			signal.Stop(sigChan)
			close(sigChan)
			close(cancelChan)
			stopped = true
			return
		case sig := <-sigChan:
			logrus.Infof("Received shutdown signal %v, terminating!", sig)
			shutdownInhibit.Lock()
			for name, handler := range handlers {
				logrus.Infof("Invoking shutdown handler %s", name)
				if err := handler(); err != nil {
					logrus.Errorf("Error running shutdown handler %s: %v", name, err)
				}
			}
			shutdownInhibit.Unlock()
			return
		}
	}()

	return nil
}

// Stop the shutdown signal handler.
func Stop() error {
	if cancelChan == nil {
		return errors.New("shutdown signal handler has not yet been started")
	}
	if stopped {
		return nil
	}

	cancelChan <- true

	return nil
}

// Temporarily inhibit signals from shutting down Libpod.
func Inhibit() {
	shutdownInhibit.RLock()
}

// Stop inhibiting signals from shutting down Libpod.
func Uninhibit() {
	shutdownInhibit.RUnlock()
}

// Register registers a function that will be executed when Podman is terminated
// by a signal.
func Register(name string, handler func() error) error {
	if handlers == nil {
		handlers = make(map[string]func() error)
	}

	if _, ok := handlers[name]; ok {
		return errors.Errorf("handler with name %s already exists", name)
	}

	handlers[name] = handler

	return nil
}

// Unregister un-registers a given shutdown handler.
func Unregister(name string) error {
	if handlers == nil {
		handlers = make(map[string]func() error)
	}

	if _, ok := handlers[name]; !ok {
		return errors.Errorf("no handler with name %s found", name)
	}

	delete(handlers, name)

	return nil
}
