package machine

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
)

type CleanupCallback struct {
	Funcs []func() error
	mu    sync.Mutex
}

func (c *CleanupCallback) CleanIfErr(err *error) {
	// Do not remove created files if the init is successful
	if *err == nil {
		return
	}
	c.clean()
}

func (c *CleanupCallback) CleanOnSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	_, ok := <-ch
	if !ok {
		return
	}

	c.clean()
	os.Exit(1)
}

func (c *CleanupCallback) clean() {
	c.mu.Lock()
	// Claim exclusive usage by copy and resetting to nil
	funcs := c.Funcs
	c.Funcs = nil
	c.mu.Unlock()

	// Already claimed or none set
	if funcs == nil {
		return
	}

	// Cleanup functions can now exclusively be run
	for _, cleanfunc := range funcs {
		if err := cleanfunc(); err != nil {
			logrus.Error(err)
		}
	}
}

func CleanUp() CleanupCallback {
	return CleanupCallback{
		Funcs: []func() error{},
	}
}

func (c *CleanupCallback) Add(anotherfunc func() error) {
	c.mu.Lock()
	c.Funcs = append(c.Funcs, anotherfunc)
	c.mu.Unlock()
}
