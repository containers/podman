package libpod

import (
	"github.com/pkg/errors"
)

// Contains the public Runtime API for pods

// A PodCreateOption is a functional option which alters the Pod created by
// NewPod
type PodCreateOption func(*Pod) error

// PodFilter is a function to determine whether a pod is included in command
// output. Pods to be outputted are tested using the function. A true return
// will include the pod, a false return will exclude it.
type PodFilter func(*Pod) bool

// NewPod makes a new, empty pod
func (r *Runtime) NewPod(options ...PodCreateOption) (*Pod, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	pod, err := newPod(r.lockDir, r)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating pod")
	}

	for _, option := range options {
		if err := option(pod); err != nil {
			return nil, errors.Wrapf(err, "error running pod create option")
		}
	}

	if pod.config.Name == "" {
		name, err := r.generateName()
		if err != nil {
			return nil, err
		}
		pod.config.Name = name
	}

	pod.valid = true

	if err := r.state.AddPod(pod); err != nil {
		return nil, errors.Wrapf(err, "error adding pod to state")
	}

	return nil, ErrNotImplemented
}

// RemovePod removes a pod
// If removeCtrs is specified, containers will be removed
// Otherwise, a pod that is not empty will return an error and not be removed
// If force is specified with removeCtrs, all containers will be stopped before
// being removed
// Otherwise, the pod will not be removed if any containers are running
func (r *Runtime) RemovePod(p *Pod, removeCtrs, force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return ErrPodRemoved
	}

	ctrs, err := r.state.PodContainers(p)
	if err != nil {
		return err
	}

	numCtrs := len(ctrs)

	if !removeCtrs && numCtrs > 0 {
		return errors.Wrapf(ErrCtrExists, "pod %s contains containers and cannot be removed", p.ID())
	}

	// Go through and lock all containers so we can operate on them all at once
	dependencies := make(map[string][]string)
	for _, ctr := range ctrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()

		// Sync all containers
		if err := ctr.syncContainer(); err != nil {
			return err
		}

		// Check if the container is in a good state to be removed
		if ctr.state.State == ContainerStatePaused {
			return errors.Wrapf(ErrCtrStateInvalid, "pod %s contains paused container %s, cannot remove", p.ID(), ctr.ID())
		}

		if ctr.state.State == ContainerStateUnknown {
			return errors.Wrapf(ErrCtrStateInvalid, "pod %s contains container %s with invalid state", p.ID(), ctr.ID())
		}

		// If the container is running and force is not set we can't do anything
		if ctr.state.State == ContainerStateRunning && !force {
			return errors.Wrapf(ErrCtrStateInvalid, "pod %s contains container %s which is running", p.ID(), ctr.ID())
		}

		// If the container has active exec sessions and force is not set we can't do anything
		if len(ctr.state.ExecSessions) != 0 && !force {
			return errors.Wrapf(ErrCtrStateInvalid, "pod %s contains container %s which has active exec sessions", p.ID(), ctr.ID())
		}

		deps, err := r.state.ContainerInUse(ctr)
		if err != nil {
			return err
		}
		dependencies[ctr.ID()] = deps
	}

	// Check if containers have dependencies
	// If they do, and the dependencies are not in the pod, error
	for ctr, deps := range dependencies {
		for _, dep := range deps {
			if _, ok := dependencies[dep]; !ok {
				return errors.Wrapf(ErrCtrExists, "container %s depends on container %s not in pod %s", ctr, dep, p.ID())
			}
		}
	}

	// First loop through all containers and stop them
	// Do not remove in this loop to ensure that we don't remove unless all
	// containers are in a good state
	if force {
		for _, ctr := range ctrs {
			// If force is set and the container is running, stop it now
			if ctr.state.State == ContainerStateRunning {
				if err := r.ociRuntime.stopContainer(ctr, ctr.StopTimeout()); err != nil {
					return errors.Wrapf(err, "error stopping container %s to remove pod %s", ctr.ID(), p.ID())
				}

				// Sync again to pick up stopped state
				if err := ctr.syncContainer(); err != nil {
					return err
				}
			}
			// If the container has active exec sessions, stop them now
			if len(ctr.state.ExecSessions) != 0 {
				if err := r.ociRuntime.execStopContainer(ctr, ctr.StopTimeout()); err != nil {
					return err
				}
			}
		}
	}

	// Start removing containers
	// We can remove containers even if they have dependencies now
	// As we have guaranteed their dependencies are in the pod
	for _, ctr := range ctrs {
		// Clean up network namespace, cgroups, mounts
		if err := ctr.cleanup(); err != nil {
			return err
		}

		// Stop container's storage
		if err := ctr.teardownStorage(); err != nil {
			return err
		}

		// Delete the container from runtime (only if we are not
		// ContainerStateConfigured)
		if ctr.state.State != ContainerStateConfigured {
			if err := r.ociRuntime.deleteContainer(ctr); err != nil {
				return errors.Wrapf(err, "error removing container %s from runtime", ctr.ID())
			}
		}

	}

	// Remove containers from the state
	if err := r.state.RemovePodContainers(p); err != nil {
		return err
	}

	// Mark containers invalid
	for _, ctr := range ctrs {
		ctr.valid = false
	}

	// Remove pod from state
	if err := r.state.RemovePod(p); err != nil {
		return err
	}

	// Mark pod invalid
	p.valid = false

	return nil
}

// GetPod retrieves a pod by its ID
func (r *Runtime) GetPod(id string) (*Pod, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	return r.state.Pod(id)
}

// HasPod checks to see if a pod with the given ID exists
func (r *Runtime) HasPod(id string) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return false, ErrRuntimeStopped
	}

	return r.state.HasPod(id)
}

// LookupPod retrieves a pod by its name or a partial ID
// If a partial ID is not unique, an error will be returned
func (r *Runtime) LookupPod(idOrName string) (*Pod, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	return r.state.LookupPod(idOrName)
}

// Pods retrieves all pods
// Filters can be provided which will determine which pods are included in the
// output. Multiple filters are handled by ANDing their output, so only pods
// matching all filters are returned
func (r *Runtime) Pods(filters ...PodFilter) ([]*Pod, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	pods, err := r.state.AllPods()
	if err != nil {
		return nil, err
	}

	podsFiltered := make([]*Pod, 0, len(pods))
	for _, pod := range pods {
		include := true
		for _, filter := range filters {
			include = include && filter(pod)
		}

		if include {
			podsFiltered = append(podsFiltered, pod)
		}
	}

	return podsFiltered, nil
}
