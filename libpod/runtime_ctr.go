package libpod

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CtrRemoveTimeout is the default number of seconds to wait after stopping a container
// before sending the kill signal
const CtrRemoveTimeout = 10

// Contains the public Runtime API for containers

// A CtrCreateOption is a functional option which alters the Container created
// by NewContainer
type CtrCreateOption func(*Container) error

// ContainerFilter is a function to determine whether a container is included
// in command output. Containers to be outputted are tested using the function.
// A true return will include the container, a false return will exclude it.
type ContainerFilter func(*Container) bool

// NewContainer creates a new container from a given OCI config
func (r *Runtime) NewContainer(rSpec *spec.Spec, options ...CtrCreateOption) (c *Container, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	ctr, err := newContainer(rSpec, r.lockDir)
	if err != nil {
		return nil, err
	}
	ctr.config.StopTimeout = CtrRemoveTimeout

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

	if ctr.config.LogPath == "" {
		ctr.config.LogPath = filepath.Join(ctr.config.StaticDir, "ctr.log")
	}
	if ctr.config.ShmDir == "" {
		ctr.config.ShmDir = filepath.Join(ctr.bundlePath(), "shm")
		if err := os.MkdirAll(ctr.config.ShmDir, 0700); err != nil {
			if !os.IsExist(err) {
				return nil, errors.Wrapf(err, "unable to create shm %q dir", ctr.config.ShmDir)
			}
		}
		ctr.config.Mounts = append(ctr.config.Mounts, ctr.config.ShmDir)
	}
	// Add the container to the state
	// TODO: May be worth looking into recovering from name/ID collisions here
	if ctr.config.Pod != "" {
		// Get the pod from state
		pod, err := r.state.Pod(ctr.config.Pod)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot add container %s to pod %s", ctr.ID(), ctr.config.Pod)
		}

		// Lock the pod to ensure we can't add containers to pods
		// being removed
		pod.lock.Lock()
		defer pod.lock.Unlock()

		if err := r.state.AddContainerToPod(pod, ctr); err != nil {
			return nil, err
		}
	} else {
		if err := r.state.AddContainer(ctr); err != nil {
			return nil, err
		}
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
	if !c.valid {
		// Container probably already removed
		// Or was never in the runtime to begin with
		return nil
	}

	// We need to lock the pod before we lock the container
	// To avoid races around removing a container and the pod it is in
	var pod *Pod
	if c.config.Pod != "" {
		pod, err := r.state.Pod(c.config.Pod)
		if err != nil {
			return errors.Wrapf(err, "container %s is in pod %s, but pod cannot be retrieved", c.ID(), pod.ID())
		}

		// Lock the pod while we're removing container
		pod.lock.Lock()
		defer pod.lock.Unlock()
	}

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
		return errors.Wrapf(ErrCtrStateInvalid, "container %s is paused, cannot remove until unpaused", c.ID())
	}

	// Check that the container's in a good state to be removed
	if c.state.State == ContainerStateRunning && force {
		if err := r.ociRuntime.stopContainer(c, c.StopTimeout()); err != nil {
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

	// Check that no other containers depend on the container
	deps, err := r.state.ContainerInUse(c)
	if err != nil {
		return err
	}
	if len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		return errors.Wrapf(ErrCtrExists, "container %s has dependent containers which must be removed before it: %s", c.ID(), depsStr)
	}

	// Stop the container's network namespace (if it has one)
	if err := r.teardownNetNS(c); err != nil {
		return err
	}

	// Stop the container's storage
	if err := c.teardownStorage(); err != nil {
		return err
	}

	// Remove the container from the state
	if c.config.Pod != "" {
		if err := r.state.RemoveContainerFromPod(pod, c); err != nil {
			return err
		}
	} else {
		if err := r.state.RemoveContainer(c); err != nil {
			return err
		}
	}

	// Delete the container
	// Only do this if we're not ContainerStateConfigured - if we are,
	// we haven't been created in the runtime yet
	if c.state.State != ContainerStateConfigured {
		if err := r.ociRuntime.deleteContainer(c); err != nil {
			return errors.Wrapf(err, "error removing container %s from runc", c.ID())
		}
	}

	// Set container as invalid so it can no longer be used
	c.valid = false

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

// GetAllContainers is a helper function for GetContainers
func (r *Runtime) GetAllContainers() ([]*Container, error) {
	return r.state.AllContainers()
}

// GetRunningContainers is a helper function for GetContainers
func (r *Runtime) GetRunningContainers() ([]*Container, error) {
	running := func(c *Container) bool {
		state, _ := c.State()
		return state == ContainerStateRunning
	}
	return r.GetContainers(running)
}

// GetContainersByList is a helper function for GetContainers
// which takes a []string of container IDs or names
func (r *Runtime) GetContainersByList(containers []string) ([]*Container, error) {
	var ctrs []*Container
	for _, inputContainer := range containers {
		ctr, err := r.LookupContainer(inputContainer)
		if err != nil {
			return ctrs, errors.Wrapf(err, "unable to lookup container %s", inputContainer)
		}
		ctrs = append(ctrs, ctr)
	}
	return ctrs, nil
}

// GetLatestContainer returns a container object of the latest created container.
func (r *Runtime) GetLatestContainer() (*Container, error) {
	lastCreatedIndex := -1
	var lastCreatedTime time.Time
	ctrs, err := r.GetAllContainers()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find latest container")
	}
	if len(ctrs) == 0 {
		return nil, ErrNoSuchCtr
	}
	for containerIndex, ctr := range ctrs {
		createdTime := ctr.config.CreatedTime
		if createdTime.After(lastCreatedTime) {
			lastCreatedTime = createdTime
			lastCreatedIndex = containerIndex
		}
	}
	return ctrs[lastCreatedIndex], nil
}
