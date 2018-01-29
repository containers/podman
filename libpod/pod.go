package libpod

import (
	"path/filepath"

	"github.com/containers/storage"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
)

// Pod represents a group of containers that may share namespaces
type Pod struct {
	id     string
	name   string
	labels map[string]string

	valid   bool
	runtime *Runtime
	lock    storage.Locker
}

// ID retrieves the pod's ID
func (p *Pod) ID() string {
	return p.id
}

// Name retrieves the pod's name
func (p *Pod) Name() string {
	return p.name
}

// Labels returns the pod's labels
func (p *Pod) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range p.labels {
		labels[key] = value
	}

	return labels
}

// Creates a new, empty pod
func newPod(lockDir string, runtime *Runtime) (*Pod, error) {
	pod := new(Pod)
	pod.id = stringid.GenerateNonCryptoID()
	pod.name = namesgenerator.GetRandomName(0)
	pod.runtime = runtime

	// Path our lock file will reside at
	lockPath := filepath.Join(lockDir, pod.id)
	// Grab a lockfile at the given path
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating lockfile for new pod")
	}
	pod.lock = lock

	return pod, nil
}

// Init initializes all containers within a pod that have not been initialized
func (p *Pod) Init() error {
	return ErrNotImplemented
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
	if !p.valid {
		return false, ErrPodRemoved
	}

	return p.runtime.state.PodHasContainer(p, id)
}

// AllContainersByID returns the container IDs of all the containers in the pod
func (p *Pod) AllContainersByID() ([]string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	return p.runtime.state.PodContainersByID(p)
}

// AllContainers retrieves the containers in the pod
func (p *Pod) AllContainers() ([]*Container, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	return p.runtime.state.PodContainers(p)
}

// Status gets the status of all containers in the pod
// TODO This should return a summary of the states of all containers in the pod
func (p *Pod) Status() error {
	return ErrNotImplemented
}
