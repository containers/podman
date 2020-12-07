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
	ErrHandlerExists error = errors.New("handler with given name already exists")
)

var (
	stopped    bool
	sigChan    chan os.Signal
	cancelChan chan bool
	// Definitions of all on-shutdown handlers
	handlers map[string]func(os.Signal) error
	// Ordering that on-shutdown handlers will be invoked.
	handlerOrder    []string
	shutdownInhibit sync.RWMutex
)

// Start begins handling SIGTERM and SIGINT and will run the given on-signal
// handlers when one is called. This can be cancelled by calling Stop().
func Start() error {
	if sigChan != nil {
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
			for _, name := range handlerOrder {
				handler, ok := handlers[name]
				if !ok {
					logrus.Errorf("Shutdown handler %s definition not found!", name)
					continue
				}
				logrus.Infof("Invoking shutdown handler %s", name)
				if err := handler(sig); err != nil {
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
// by a signal. Handlers are invoked LIFO - the last handler registered is the
// first run.
func Register(name string, handler func(os.Signal) error) error {
	if handlers == nil {
		handlers = make(map[string]func(os.Signal) error)
	}

	if _, ok := handlers[name]; ok {
		return ErrHandlerExists
	}

	handlers[name] = handler
	handlerOrder = append([]string{name}, handlerOrder...)

	return nil
}

// Unregister un-registers a given shutdown handler.
func Unregister(name string) error {
	if handlers == nil {
		return nil
	}

	if _, ok := handlers[name]; !ok {
		return nil
	}

	delete(handlers, name)

	newOrder := []string{}
	for _, checkName := range handlerOrder {
		if checkName != name {
			newOrder = append(newOrder, checkName)
		}
	}
	handlerOrder = newOrder

	return nil
}
