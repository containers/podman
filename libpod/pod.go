package libpod

import (
	"path/filepath"

	"github.com/containers/storage"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Pod represents a group of containers that may share namespaces
type Pod struct {
	config *PodConfig

	valid   bool
	runtime *Runtime
	lock    storage.Locker
}

// PodConfig represents a pod's static configuration
type PodConfig struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Labels map[string]string `json:""`
}

// ID retrieves the pod's ID
func (p *Pod) ID() string {
	return p.config.ID
}

// Name retrieves the pod's name
func (p *Pod) Name() string {
	return p.config.Name
}

// Labels returns the pod's labels
func (p *Pod) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range p.config.Labels {
		labels[key] = value
	}

	return labels
}

// Creates a new, empty pod
func newPod(lockDir string, runtime *Runtime) (*Pod, error) {
	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.config.ID = stringid.GenerateNonCryptoID()
	pod.config.Name = namesgenerator.GetRandomName(0)
	pod.config.Labels = make(map[string]string)
	pod.runtime = runtime

	// Path our lock file will reside at
	lockPath := filepath.Join(lockDir, pod.config.ID)
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

// Start starts all containers within a pod
// Containers that are already running or have been paused are ignored
// If an error is encountered starting any container, Start() will cease
// starting containers and immediately report an error
// Start() is not an atomic operation - if an error is reported, containers that
// have already started will remain running
func (p *Pod) Start() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return err
	}

	// We need to lock all the containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()

		if err := ctr.syncContainer(); err != nil {
			return err
		}
	}

	// Send a signal to all containers
	for _, ctr := range allCtrs {
		// Ignore containers that are not created or stopped
		if ctr.state.State != ContainerStateCreated && ctr.state.State != ContainerStateStopped {
			continue
		}

		// TODO remove this when we patch conmon to support restarting containers
		if ctr.state.State == ContainerStateStopped {
			continue;
		}

		if err := ctr.runtime.ociRuntime.startContainer(ctr); err != nil {
			return errors.Wrapf(err, "error starting container %s", ctr.ID())
		}

		// We can safely assume the container is running
		ctr.state.State = ContainerStateRunning

		if err := ctr.save(); err != nil {
			return err
		}

		logrus.Debugf("Started container %s", ctr.ID())
	}

	return nil
}

// Stop stops all containers within a pod that are not already stopped
// Each container will use its own stop timeout
// Only running containers will be stopped. Paused, stopped, or created
// containers will be ignored.
// If an error is encountered stopping any one container, no further containers
// will be stopped, and an error will immediately be returned.
// Stop() is not an atomic operation - if an error is encountered, containers
// which have already been stopped will not be restarted
func (p *Pod) Stop() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return err
	}

	// We need to lock all the containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()

		if err := ctr.syncContainer(); err != nil {
			return err
		}
	}

	// Send a signal to all containers
	for _, ctr := range allCtrs {
		// Ignore containers that are not running
		if ctr.state.State != ContainerStateRunning {
			continue
		}

		if err := ctr.runtime.ociRuntime.stopContainer(ctr, ctr.config.StopTimeout); err != nil {
			return errors.Wrapf(err, "error stopping container %s", ctr.ID())
		}

		// Sync container state to pick up return code
		if err := ctr.runtime.ociRuntime.updateContainerStatus(ctr); err != nil {
			return err
		}

		// Clean up storage to ensure we don't leave dangling mounts
		if err := ctr.cleanupStorage(); err != nil {
			return err
		}

		logrus.Debugf("Stopped container %s", ctr.ID())
	}

	return nil
}

// Kill sends a signal to all running containers within a pod
// Signals will only be sent to running containers. Containers that are not
// running will be ignored.
// If an error is encountered signalling any one container, kill will stop
// and immediately return an error, sending no further signals
// Kill() is not an atomic operation - if an error is encountered, no further
// signals will be sent, but some signals may already have been sent
func (p *Pod) Kill(signal uint) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return err
	}

	// We need to lock all the containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()

		if err := ctr.syncContainer(); err != nil {
			return err
		}
	}

	// Send a signal to all containers
	for _, ctr := range allCtrs {
		// Ignore containers that are not running
		if ctr.state.State != ContainerStateRunning {
			continue
		}

		if err := ctr.runtime.ociRuntime.killContainer(ctr, signal); err != nil {
			return errors.Wrapf(err, "error killing container %s", ctr.ID())
		}

		logrus.Debugf("Killed container %s with signal %d", ctr.ID(), signal)
	}

	return nil
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
// Returns a map of Container ID to Container Status
func (p *Pod) Status() (map[string]ContainerStatus, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	// We need to lock all the containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()
	}

	// Now that all containers are locked, get their status
	status := make(map[string]ContainerStatus, len(allCtrs))
	for _, ctr := range allCtrs {
		if err := ctr.syncContainer(); err != nil {
			return nil, err
		}

		status[ctr.ID()] = ctr.state.State
	}

	return status, nil
}

// TODO add pod batching
// Lock pod to avoid lock contention
// Store and lock all containers (no RemoveContainer in batch guarantees cache will not become stale)
