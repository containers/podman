// +build !remoteclient

package adapter

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetLatestContainer gets the latest Container and wraps it in an adapter Container
func (r *LocalRuntime) GetLatestContainer() (*Container, error) {
	Container := Container{}
	c, err := r.Runtime.GetLatestContainer()
	Container.Container = c
	return &Container, err
}

// GetAllContainers gets all Containers and wraps each one in an adapter Container
func (r *LocalRuntime) GetAllContainers() ([]*Container, error) {
	var containers []*Container
	allContainers, err := r.Runtime.GetAllContainers()
	if err != nil {
		return nil, err
	}

	for _, c := range allContainers {
		containers = append(containers, &Container{c})
	}
	return containers, nil
}

// LookupContainer gets a Container by name or id and wraps it in an adapter Container
func (r *LocalRuntime) LookupContainer(idOrName string) (*Container, error) {
	ctr, err := r.Runtime.LookupContainer(idOrName)
	if err != nil {
		return nil, err
	}
	return &Container{ctr}, nil
}

// StopContainers stops container(s) based on CLI inputs.
// Returns list of successful id(s), map of failed id(s) + error, or error not from container
func (r *LocalRuntime) StopContainers(ctx context.Context, cli *cliconfig.StopValues) ([]string, map[string]error, error) {
	var timeout *uint
	if cli.Flags().Changed("timeout") || cli.Flags().Changed("time") {
		t := uint(cli.Timeout)
		timeout = &t
	}

	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return ok, failures, err
	}

	for _, c := range ctrs {
		if timeout == nil {
			t := c.StopTimeout()
			timeout = &t
			logrus.Debugf("Set timeout to container %s default (%d)", c.ID(), *timeout)
		}
		if err := c.StopWithTimeout(*timeout); err == nil {
			ok = append(ok, c.ID())
		} else if errors.Cause(err) == libpod.ErrCtrStopped {
			ok = append(ok, c.ID())
			logrus.Debugf("Container %s is already stopped", c.ID())
		} else {
			failures[c.ID()] = err
		}
	}
	return ok, failures, nil
}

// KillContainers sends signal to container(s) based on CLI inputs.
// Returns list of successful id(s), map of failed id(s) + error, or error not from container
func (r *LocalRuntime) KillContainers(ctx context.Context, cli *cliconfig.KillValues, signal syscall.Signal) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return ok, failures, err
	}

	for _, c := range ctrs {
		if err := c.Kill(uint(signal)); err == nil {
			ok = append(ok, c.ID())
		} else {
			failures[c.ID()] = err
		}
	}
	return ok, failures, nil
}

// WaitOnContainers waits for all given container(s) to stop
func (r *LocalRuntime) WaitOnContainers(ctx context.Context, cli *cliconfig.WaitValues, interval time.Duration) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ctrs, err := shortcuts.GetContainersByContext(false, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return ok, failures, err
	}

	for _, c := range ctrs {
		if returnCode, err := c.WaitWithInterval(interval); err == nil {
			ok = append(ok, strconv.Itoa(int(returnCode)))
		} else {
			failures[c.ID()] = err
		}
	}
	return ok, failures, err
}

// Log logs one or more containers
func (r *LocalRuntime) Log(c *cliconfig.LogsValues, options *libpod.LogOptions) error {
	var wg sync.WaitGroup
	options.WaitGroup = &wg
	if len(c.InputArgs) > 1 {
		options.Multi = true
	}
	logChannel := make(chan *libpod.LogLine, int(c.Tail)*len(c.InputArgs)+1)
	containers, err := shortcuts.GetContainersByContext(false, c.Latest, c.InputArgs, r.Runtime)
	if err != nil {
		return err
	}
	if err := r.Runtime.Log(containers, options, logChannel); err != nil {
		return err
	}
	go func() {
		wg.Wait()
		close(logChannel)
	}()
	for line := range logChannel {
		fmt.Println(line.String(options))
	}
	return nil
}
