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

	pod, err := newPod()
	if err != nil {
		return nil, errors.Wrapf(err, "error creating pod")
	}

	for _, option := range options {
		if err := option(pod); err != nil {
			return nil, errors.Wrapf(err, "error running pod create option")
		}
	}

	pod.valid = true

	if err := r.state.AddPod(pod); err != nil {
		return nil, errors.Wrapf(err, "error adding pod to state")
	}

	return nil, ErrNotImplemented
}

// RemovePod removes a pod and all containers in it
// If force is specified, all containers in the pod will be stopped first
// Otherwise, RemovePod will return an error if any container in the pod is running
// Remove acts atomically, removing all containers or no containers
func (r *Runtime) RemovePod(p *Pod, force bool) error {
	return ErrNotImplemented
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
