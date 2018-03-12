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
// ffjson: skip
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

// TODO: need function to produce a directed graph of containers
// This would allow us to properly determine stop/start order

// Start starts all containers within a pod
// It combines the effects of Init() and Start() on a container
// If a container has already been initialized it will be started,
// otherwise it will be initialized then started.
// Containers that are already running or have been paused are ignored
// All containers are started independently, in order dictated by their
// dependencies.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were started
// If map is not nil, an error was encountered when starting one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were started successfully
func (p *Pod) Start() (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	// Maintain a map of containers still to start
	ctrsToStart := make(map[string]*Container)
	// Maintain a map of all containers so we can easily look up dependencies
	allCtrsMap := make(map[string]*Container)

	// We need to lock all the containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()

		if err := ctr.syncContainer(); err != nil {
			return nil, err
		}

		if ctr.state.State == ContainerStateConfigured ||
			ctr.state.State == ContainerStateCreated ||
			ctr.state.State == ContainerStateStopped {
			ctrsToStart[ctr.ID()] = ctr
		}
		allCtrsMap[ctr.ID()] = ctr
	}

	ctrErrors := make(map[string]error)

	// Loop at most 10 times, to prevent potential infinite loops in
	// dependencies
	loopCounter := 10

	// Loop while we still have containers to start
	for len(ctrsToStart) > 0 {
		// Loop through all containers, attempting to start them
		for id, ctr := range ctrsToStart {
			// TODO remove this when we support restarting containers
			if ctr.state.State == ContainerStateStopped {
				ctrErrors[id] = errors.Wrapf(ErrNotImplemented, "starting stopped containers is not yet supported")

				delete(ctrsToStart, id)
				continue
			}

			// TODO should we only do a dependencies check if we are not ContainerStateCreated?
			depsOK := true
			var depErr error
			// Check container dependencies
			for _, depID := range ctr.Dependencies() {
				depCtr := allCtrsMap[depID]
				if depCtr.state.State != ContainerStateRunning &&
					depCtr.state.State != ContainerStatePaused {
					// We are definitely not OK to init, a dependency is not up
					depsOK = false
					// Check to see if the dependency errored
					// If it did, error here too
					if _, ok := ctrErrors[depID]; ok {
						depErr = errors.Wrapf(ErrCtrStateInvalid, "dependency %s of container %s failed to start", depID, id)
					}

					break
				}
			}
			if !depsOK {
				// Only if one of the containers dependencies failed should we stop trying
				// Otherwise, assume it's just yet to start, retry starting this container later
				if depErr != nil {
					ctrErrors[id] = depErr
					delete(ctrsToStart, id)
				}
				continue
			}

			// Initialize and start the container
			if err := ctr.initAndStart(); err != nil {
				ctrErrors[id] = err
			}
			delete(ctrsToStart, id)
		}

		loopCounter = loopCounter - 1
		if loopCounter == 0 {
			// Loop through all remaining containers and add an error
			for id := range ctrsToStart {
				ctrErrors[id] = errors.Wrapf(ErrInternal, "exceeded maximum attempts trying to start container %s", id)
			}

			break
		}
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(ErrCtrExists, "error starting some containers")
	}

	return nil, nil
}

// Stop stops all containers within a pod that are not already stopped
// Each container will use its own stop timeout
// Only running containers will be stopped. Paused, stopped, or created
// containers will be ignored.
// If cleanup is true, mounts and network namespaces will be cleaned up after
// the container is stopped.
// All containers are stopped independently. An error stopping one container
// will not prevent other containers being stopped.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were stopped
// If map is not nil, an error was encountered when stopping one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were stopped without error
func (p *Pod) Stop(cleanup bool) (map[string]error, error) {
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

		if err := ctr.syncContainer(); err != nil {
			return nil, err
		}
	}

	ctrErrors := make(map[string]error)

	// TODO: There may be cases where it makes sense to order stops based on
	// dependencies. Should we bother with this?

	// Stop to all containers
	for _, ctr := range allCtrs {
		// Ignore containers that are not running
		if ctr.state.State != ContainerStateRunning {
			continue
		}

		if err := ctr.stop(ctr.config.StopTimeout); err != nil {
			ctrErrors[ctr.ID()] = err
			continue
		}

		if cleanup {
			// Clean up storage to ensure we don't leave dangling mounts
			if err := ctr.cleanupStorage(); err != nil {
				ctrErrors[ctr.ID()] = err
				continue
			}

			// Clean up network namespace
			if err := ctr.cleanupNetwork(); err != nil {
				ctrErrors[ctr.ID()] = err
			}
		}
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(ErrCtrExists, "error stopping some containers")
	}

	return nil, nil
}

// Kill sends a signal to all running containers within a pod
// Signals will only be sent to running containers. Containers that are not
// running will be ignored. All signals are sent independently, and sending will
// continue even if some containers encounter errors.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were signalled
// If map is not nil, an error was encountered when signalling one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were signalled successfully
func (p *Pod) Kill(signal uint) (map[string]error, error) {
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

		if err := ctr.syncContainer(); err != nil {
			return nil, err
		}
	}

	ctrErrors := make(map[string]error)

	// Send a signal to all containers
	for _, ctr := range allCtrs {
		// Ignore containers that are not running
		if ctr.state.State != ContainerStateRunning {
			continue
		}

		if err := ctr.runtime.ociRuntime.killContainer(ctr, signal); err != nil {
			ctrErrors[ctr.ID()] = err
			continue
		}

		logrus.Debugf("Killed container %s with signal %d", ctr.ID(), signal)
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, nil
	}

	return nil, nil
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
