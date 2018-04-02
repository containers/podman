package libpod

import (
	"path/filepath"

	"github.com/containers/storage"
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

	// We need to lock all the containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()

		if err := ctr.syncContainer(); err != nil {
			return nil, err
		}
	}

	// Build a dependency graph of containers in the pod
	graph, err := buildContainerGraph(allCtrs)
	if err != nil {
		return nil, errors.Wrapf(err, "error generating dependency graph for pod %s", p.ID())
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	// If there are no containers without dependencies, we can't start
	// Error out
	if len(graph.noDepNodes) == 0 {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no containers in pod %s have no dependencies, cannot start pod", p.ID())
	}

	// Traverse the graph beginning at nodes with no dependencies
	for _, node := range graph.noDepNodes {
		startNode(node, false, ctrErrors, ctrsVisited)
	}

	return ctrErrors, nil
}

// Visit a node on a container graph and start the container, or set an error if
// a dependency failed to start
func startNode(node *containerNode, setError bool, ctrErrors map[string]error, ctrsVisited map[string]bool) {
	// First, check if we have already visited the node
	if ctrsVisited[node.id] {
		return
	}

	// If setError is true, a dependency of us failed
	// Mark us as failed and recurse
	if setError {
		// Mark us as visited, and set an error
		ctrsVisited[node.id] = true
		ctrErrors[node.id] = errors.Wrapf(ErrCtrStateInvalid, "a dependency of container %s failed to start", node.id)

		// Hit anyone who depends on us, and set errors on them too
		for _, successor := range node.dependedOn {
			startNode(successor, true, ctrErrors, ctrsVisited)
		}

		return
	}

	// Have all our dependencies started?
	// If not, don't visit the node yet
	depsVisited := true
	for _, dep := range node.dependsOn {
		depsVisited = depsVisited && ctrsVisited[dep.id]
	}
	if !depsVisited {
		// Don't visit us yet, all dependencies are not up
		// We'll hit the dependencies eventually, and when we do it will
		// recurse here
		return
	}

	// Going to start the container, mark us as visited
	ctrsVisited[node.id] = true

	// TODO: Maybe have a checkDependenciesRunningLocked here?
	// Graph traversal should ensure our deps have been started, but some
	// might have stopped since?
	// Potentially will hurt our perf, though

	// Start the container (only if it is not running)
	ctrErrored := false
	if node.container.state.State != ContainerStateRunning {
		if err := node.container.initAndStart(); err != nil {
			ctrErrored = true
			ctrErrors[node.id] = err
		}
	}

	// Recurse to anyone who depends on us and start them
	for _, successor := range node.dependedOn {
		startNode(successor, ctrErrored, ctrErrors, ctrsVisited)
	}

	return
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
			if err := ctr.cleanup(); err != nil {
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
