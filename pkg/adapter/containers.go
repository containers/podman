// +build !remoteclient

package adapter

import (
	"context"
	"strconv"
	"syscall"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
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

	maxWorkers := shared.DefaultPoolSize("stop")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum stop workers to %d", maxWorkers)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return nil, nil, err
	}

	pool := shared.NewPool("stop", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		if timeout == nil {
			t := c.StopTimeout()
			timeout = &t
			logrus.Debugf("Set stop timeout to container %s default (%d)", c.ID(), *timeout)
		}

		pool.Add(shared.Job{
			c.ID(),
			func() error {
				err := c.StopWithTimeout(*timeout)
				if err != nil {
					if errors.Cause(err) == libpod.ErrCtrStopped {
						logrus.Debugf("Container %s is already stopped", c.ID())
						return nil
					}
				}
				return err
			},
		})
	}
	return pool.Run()
}

// KillContainers sends signal to container(s) based on CLI inputs.
// Returns list of successful id(s), map of failed id(s) + error, or error not from container
func (r *LocalRuntime) KillContainers(ctx context.Context, cli *cliconfig.KillValues, signal syscall.Signal) ([]string, map[string]error, error) {
	maxWorkers := shared.DefaultPoolSize("kill")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum kill workers to %d", maxWorkers)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return nil, nil, err
	}

	pool := shared.NewPool("kill", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		pool.Add(shared.Job{
			c.ID(),
			func() error {
				return c.Kill(uint(signal))
			},
		})
	}
	return pool.Run()
}

// RemoveContainers removes container(s) based on CLI inputs.
func (r *LocalRuntime) RemoveContainers(ctx context.Context, cli *cliconfig.RmValues) ([]string, map[string]error, error) {
	maxWorkers := shared.DefaultPoolSize("rm")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum rm workers to %d", maxWorkers)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		// Force may be used to remove containers no longer found in the database
		if cli.Force && len(cli.InputArgs) > 0 && errors.Cause(err) == libpod.ErrNoSuchCtr {
			r.RemoveContainersFromStorage(cli.InputArgs)
		}
		return nil, nil, err
	}

	pool := shared.NewPool("rm", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		pool.Add(shared.Job{
			c.ID(),
			func() error {
				return r.RemoveContainer(ctx, c, cli.Force, cli.Volumes)
			},
		})
	}
	return pool.Run()
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
