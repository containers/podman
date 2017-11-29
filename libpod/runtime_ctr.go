package libpod

import (
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const ctrRemoveTimeout = 10

// Contains the public Runtime API for containers

// A CtrCreateOption is a functional option which alters the Container created
// by NewContainer
type CtrCreateOption func(*Container) error

// ContainerFilter is a function to determine whether a container is included
// in command output. Containers to be outputted are tested using the function.
// A true return will include the container, a false return will exclude it.
type ContainerFilter func(*Container) bool

// NewContainer creates a new container from a given OCI config
func (r *Runtime) NewContainer(spec *spec.Spec, options ...CtrCreateOption) (c *Container, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	ctr, err := newContainer(spec)
	if err != nil {
		return nil, err
	}

	for _, option := range options {
		if err := option(ctr); err != nil {
			return nil, errors.Wrapf(err, "error running container create option")
		}
	}

	ctr.valid = true
	ctr.state.State = ContainerStateConfigured
	ctr.runtime = r

	// Set up storage for the container
	if err := ctr.setupStorage(); err != nil {
		return nil, errors.Wrapf(err, "error configuring storage for container")
	}
	defer func() {
		if err != nil {
			if err2 := ctr.teardownStorage(); err2 != nil {
				logrus.Errorf("Error removing partially-created container root filesystem: %s", err2)
			}
		}
	}()

	// If the container is in a pod, add it to the pod
	if ctr.pod != nil {
		if err := ctr.pod.addContainer(ctr); err != nil {
			return nil, errors.Wrapf(err, "error adding new container to pod %s", ctr.pod.ID())
		}
	}
	defer func() {
		if err != nil && ctr.pod != nil {
			if err2 := ctr.pod.removeContainer(ctr); err2 != nil {
				logrus.Errorf("Error removing partially-created container from pod %s: %s", ctr.pod.ID(), err2)
			}
		}
	}()

	if err := r.state.AddContainer(ctr); err != nil {
		// TODO: Might be worth making an effort to detect duplicate IDs
		// We can recover from that by generating a new ID for the
		// container
		return nil, errors.Wrapf(err, "error adding new container to state")
	}

	return ctr, nil
}

// RemoveContainer removes the given container
// If force is specified, the container will be stopped first
// Otherwise, RemoveContainer will return an error if the container is running
func (r *Runtime) RemoveContainer(c *Container, force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.removeContainer(c, force)
}

// Internal function to remove a container
// Locks the container, but does not lock the runtime
func (r *Runtime) removeContainer(c *Container, force bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	// Update the container to get current state
	if err := c.syncContainer(); err != nil {
		return err
	}

	if c.state.State == ContainerStatePaused {
		return errors.Wrapf("container %s is paused, cannot remove until unpaused", c.ID())
	}

	// Check that the container's in a good state to be removed
	if c.state.State == ContainerStateRunning && force {
		if err := r.ociRuntime.stopContainer(c, ctrRemoveTimeout); err != nil {
			return errors.Wrapf(err, "cannot remove container %s as it could not be stopped", c.ID())
		}

		// Need to update container state to make sure we know it's stopped
		if err := c.syncContainer(); err != nil {
			return err
		}
	} else if !(c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateCreated ||
		c.state.State == ContainerStateStopped) {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot remove container %s as it is %s - running or paused containers cannot be removed", c.ID(), c.state.State.String())
	}

	// Stop the container's storage
	if err := c.teardownStorage(); err != nil {
		return err
	}

	if err := r.state.RemoveContainer(c); err != nil {
		return errors.Wrapf(err, "error removing container from state")
	}

	// Delete the container
	if err := r.ociRuntime.deleteContainer(c); err != nil {
		return errors.Wrapf(err, "error removing container %s from runc", c.ID())
	}

	// Set container as invalid so it can no longer be used
	c.valid = false

	// Remove container from pod, if it joined one
	if c.pod != nil {
		if err := c.pod.removeContainer(c); err != nil {
			return errors.Wrapf(err, "error removing container from pod %s", c.pod.ID())
		}
	}

	return nil
}

// GetContainer retrieves a container by its ID
func (r *Runtime) GetContainer(id string) (*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	return r.state.Container(id)
}

// HasContainer checks if a container with the given ID is present
func (r *Runtime) HasContainer(id string) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return false, ErrRuntimeStopped
	}

	return r.state.HasContainer(id)
}

// LookupContainer looks up a container by its name or a partial ID
// If a partial ID is not unique, an error will be returned
func (r *Runtime) LookupContainer(idOrName string) (*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	return r.state.LookupContainer(idOrName)
}

// GetContainers retrieves all containers from the state
// Filters can be provided which will determine what containers are included in
// the output. Multiple filters are handled by ANDing their output, so only
// containers matching all filters are returned
func (r *Runtime) GetContainers(filters ...ContainerFilter) ([]*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	ctrs, err := r.state.AllContainers()
	if err != nil {
		return nil, err
	}

	ctrsFiltered := make([]*Container, 0, len(ctrs))

	for _, ctr := range ctrs {
		include := true
		for _, filter := range filters {
			include = include && filter(ctr)
		}

		if include {
			ctrsFiltered = append(ctrsFiltered, ctr)
		}
	}

	return ctrsFiltered, nil
}
