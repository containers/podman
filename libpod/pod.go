package libpod

import (
	"os"
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

	containers map[string]*Container

	valid bool
	lock  storage.Locker
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
func newPod(lockDir string) (*Pod, error) {
	pod := new(Pod)
	pod.id = stringid.GenerateNonCryptoID()
	pod.name = namesgenerator.GetRandomName(0)

	pod.containers = make(map[string]*Container)

	// TODO: containers and pods share a locks folder, but not tables in the
	// database
	// As the locks are 256-bit pseudorandom integers, collision is unlikely
	// But it's something worth looking into

	// Path our lock file will reside at
	lockPath := filepath.Join(lockDir, pod.id)
	// Ensure there is no conflict - file does not exist
	_, err := os.Stat(lockPath)
	if err == nil || !os.IsNotExist(err) {
		return nil, errors.Wrapf(ErrCtrExists, "lockfile for pod ID %s already exists", pod.id)
	}
	// Grab a lockfile at the given path
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating lockfile for new pod")
	}
	pod.lock = lock

	return pod, nil
}

// Adds a container to the pod
// Does not check that container's pod ID is set correctly, or attempt to set
// pod ID after adding
func (p *Pod) addContainer(ctr *Container) error {
	p.lock.Lock()
	defer p.lock.Unlock()

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
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return false, ErrPodRemoved
	}

	_, ok := p.containers[id]

	return ok, nil
}

// GetContainers retrieves the containers in the pod
func (p *Pod) GetContainers() ([]*Container, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

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
