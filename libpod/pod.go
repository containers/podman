package libpod

import (
	"sync"

	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
)

// Pod represents a group of containers that may share namespaces
type Pod struct {
	id   string
	name string

	containers map[string]*Container

	valid bool
	lock  sync.RWMutex
}

// ID retrieves the pod's ID
func (p *Pod) ID() string {
	return p.id
}

// Name retrieves the pod's name
func (p *Pod) Name() string {
	return p.name
}

// Creates a new pod
func newPod() (*Pod, error) {
	pod := new(Pod)
	pod.id = stringid.GenerateNonCryptoID()
	pod.name = pod.id // TODO generate human-readable name here

	pod.containers = make(map[string]*Container)

	return pod, nil
}

// Adds a container to the pod
// Does not check that container's pod ID is set correctly, or attempt to set
// pod ID after adding
func (p *Pod) addContainer(ctr *Container) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	ctr.lock.Lock()
	defer ctr.lock.Unlock()

	if !p.valid {
		return ErrPodRemoved
	}

	if !ctr.valid {
		return ErrCtrRemoved
	}

	if _, ok := p.containers[ctr.ID()]; ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in pod %s", ctr.ID(), p.id)
	}

	p.containers[ctr.ID()] = ctr

	return nil
}

// Removes a container from the pod
// Does not perform any checks on the container
func (p *Pod) removeContainer(ctr *Container) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return ErrPodRemoved
	}

	if _, ok := p.containers[ctr.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchCtr, "no container with id %s in pod %s", ctr.ID(), p.id)
	}

	delete(p.containers, ctr.ID())

	return nil
}

// Start starts all containers within a pod that are not already running
func (p *Pod) Start() error {
	return ErrNotImplemented
}

// Stop stops all containers within a pod that are not already stopped
func (p *Pod) Stop() error {
	return ErrNotImplemented
}

// Kill sends a signal to all running containers within a pod
func (p *Pod) Kill(signal uint) error {
	return ErrNotImplemented
}

// HasContainer checks if a container is present in the pod
func (p *Pod) HasContainer(id string) (bool, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	if !p.valid {
		return false, ErrPodRemoved
	}

	_, ok := p.containers[id]

	return ok, nil
}

// GetContainers retrieves the containers in the pod
func (p *Pod) GetContainers() ([]*Container, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	ctrs := make([]*Container, 0, len(p.containers))
	for _, ctr := range p.containers {
		ctrs = append(ctrs, ctr)
	}

	return ctrs, nil
}

// Status gets the status of all containers in the pod
// TODO This should return a summary of the states of all containers in the pod
func (p *Pod) Status() error {
	return ErrNotImplemented
}
