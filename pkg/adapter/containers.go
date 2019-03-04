// +build !remoteclient

package adapter

import (
	"context"
	"syscall"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	"github.com/pkg/errors"
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
	timeout := uint(0)
	if cli.Flags().Changed("timeout") {
		timeout = uint(cli.Timeout)
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
		err := c.StopWithTimeout(timeout)
		if err != nil && errors.Cause(err) != libpod.ErrCtrStopped {
			failures[c.ID()] = err
		} else {
			ok = append(ok, c.ID())
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
