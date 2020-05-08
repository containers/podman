package libpod

import (
	"context"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

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
func (p *Pod) Start(ctx context.Context) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	// Build a dependency graph of containers in the pod
	graph, err := BuildContainerGraph(allCtrs)
	if err != nil {
		return nil, errors.Wrapf(err, "error generating dependency graph for pod %s", p.ID())
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	// If there are no containers without dependencies, we can't start
	// Error out
	if len(graph.noDepNodes) == 0 {
		return nil, errors.Wrapf(define.ErrNoSuchCtr, "no containers in pod %s have no dependencies, cannot start pod", p.ID())
	}

	// Traverse the graph beginning at nodes with no dependencies
	for _, node := range graph.noDepNodes {
		startNode(ctx, node, false, ctrErrors, ctrsVisited, false)
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrCtrExists, "error starting some containers")
	}
	defer p.newPodEvent(events.Start)
	return nil, nil
}

// Stop stops all containers within a pod without a timeout.  It assumes -1 for
// a timeout.
func (p *Pod) Stop(ctx context.Context, cleanup bool) (map[string]error, error) {
	return p.StopWithTimeout(ctx, cleanup, -1)
}

// StopWithTimeout stops all containers within a pod that are not already stopped
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
func (p *Pod) StopWithTimeout(ctx context.Context, cleanup bool, timeout int) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrors := make(map[string]error)

	// TODO: There may be cases where it makes sense to order stops based on
	// dependencies. Should we bother with this?

	// Stop to all containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()

		if err := ctr.syncContainer(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		// Ignore containers that are not running
		if ctr.state.State != define.ContainerStateRunning {
			ctr.lock.Unlock()
			continue
		}
		stopTimeout := ctr.config.StopTimeout
		if timeout > -1 {
			stopTimeout = uint(timeout)
		}
		if err := ctr.stop(stopTimeout); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		if cleanup {
			if err := ctr.cleanup(ctx); err != nil {
				ctrErrors[ctr.ID()] = err
			}
		}

		ctr.lock.Unlock()
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrCtrExists, "error stopping some containers")
	}
	defer p.newPodEvent(events.Stop)
	return nil, nil
}

// Pause pauses all containers within a pod that are running.
// Only running containers will be paused. Paused, stopped, or created
// containers will be ignored.
// All containers are paused independently. An error pausing one container
// will not prevent other containers being paused.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were paused
// If map is not nil, an error was encountered when pausing one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were paused without error
func (p *Pod) Pause() (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	if rootless.IsRootless() {
		cgroupv2, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine cgroupversion")
		}
		if !cgroupv2 {
			return nil, errors.Wrap(define.ErrNoCgroups, "can not pause pods containing rootless containers with cgroup V1")
		}
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrors := make(map[string]error)

	// Pause to all containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()

		if err := ctr.syncContainer(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		// Ignore containers that are not running
		if ctr.state.State != define.ContainerStateRunning {
			ctr.lock.Unlock()
			continue
		}

		if err := ctr.pause(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		ctr.lock.Unlock()
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrCtrExists, "error pausing some containers")
	}
	defer p.newPodEvent(events.Pause)
	return nil, nil
}

// Unpause unpauses all containers within a pod that are running.
// Only paused containers will be unpaused. Running, stopped, or created
// containers will be ignored.
// All containers are unpaused independently. An error unpausing one container
// will not prevent other containers being unpaused.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were unpaused
// If map is not nil, an error was encountered when unpausing one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were unpaused without error
func (p *Pod) Unpause() (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrors := make(map[string]error)

	// Pause to all containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()

		if err := ctr.syncContainer(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		// Ignore containers that are not paused
		if ctr.state.State != define.ContainerStatePaused {
			ctr.lock.Unlock()
			continue
		}

		if err := ctr.unpause(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		ctr.lock.Unlock()
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrCtrExists, "error unpausing some containers")
	}

	defer p.newPodEvent(events.Unpause)
	return nil, nil
}

// Restart restarts all containers within a pod that are not paused or in an error state.
// It combines the effects of Stop() and Start() on a container
// Each container will use its own stop timeout.
// All containers are started independently, in order dictated by their
// dependencies. An error restarting one container
// will not prevent other containers being restarted.
// An error and a map[string]error are returned
// If the error is not nil and the map is nil, an error was encountered before
// any containers were restarted
// If map is not nil, an error was encountered when restarting one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrCtrExists
// If both error and the map are nil, all containers were restarted without error
func (p *Pod) Restart(ctx context.Context) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	// Build a dependency graph of containers in the pod
	graph, err := BuildContainerGraph(allCtrs)
	if err != nil {
		return nil, errors.Wrapf(err, "error generating dependency graph for pod %s", p.ID())
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	// If there are no containers without dependencies, we can't start
	// Error out
	if len(graph.noDepNodes) == 0 {
		return nil, errors.Wrapf(define.ErrNoSuchCtr, "no containers in pod %s have no dependencies, cannot start pod", p.ID())
	}

	// Traverse the graph beginning at nodes with no dependencies
	for _, node := range graph.noDepNodes {
		startNode(ctx, node, false, ctrErrors, ctrsVisited, true)
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrCtrExists, "error stopping some containers")
	}
	p.newPodEvent(events.Stop)
	p.newPodEvent(events.Start)
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
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrors := make(map[string]error)

	// Send a signal to all containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()

		if err := ctr.syncContainer(); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		// Ignore containers that are not running
		if ctr.state.State != define.ContainerStateRunning {
			ctr.lock.Unlock()
			continue
		}

		if err := ctr.ociRuntime.KillContainer(ctr, signal, false); err != nil {
			ctr.lock.Unlock()
			ctrErrors[ctr.ID()] = err
			continue
		}

		logrus.Debugf("Killed container %s with signal %d", ctr.ID(), signal)

		ctr.state.StoppedByUser = true
		if err := ctr.save(); err != nil {
			ctrErrors[ctr.ID()] = err
		}

		ctr.lock.Unlock()
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrCtrExists, "error killing some containers")
	}
	defer p.newPodEvent(events.Kill)
	return nil, nil
}

// Status gets the status of all containers in the pod
// Returns a map of Container ID to Container Status
func (p *Pod) Status() (map[string]define.ContainerStatus, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}
	return containerStatusFromContainers(allCtrs)
}

func containerStatusFromContainers(allCtrs []*Container) (map[string]define.ContainerStatus, error) {
	// We need to lock all the containers
	for _, ctr := range allCtrs {
		ctr.lock.Lock()
		defer ctr.lock.Unlock()
	}

	// Now that all containers are locked, get their status
	status := make(map[string]define.ContainerStatus, len(allCtrs))
	for _, ctr := range allCtrs {
		if err := ctr.syncContainer(); err != nil {
			return nil, err
		}

		status[ctr.ID()] = ctr.state.State
	}

	return status, nil
}

// Inspect returns a PodInspect struct to describe the pod
func (p *Pod) Inspect() (*define.InspectPodData, error) {
	var (
		ctrs []define.InspectPodContainerInfo
	)

	p.lock.Lock()
	defer p.lock.Unlock()
	if err := p.updatePod(); err != nil {
		return nil, err
	}

	containers, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}
	ctrStatuses := make(map[string]define.ContainerStatus, len(containers))
	for _, c := range containers {
		containerStatus := "unknown"
		// Ignoring possible errors here because we don't want this to be
		// catastrophic in nature
		containerState, err := c.State()
		if err == nil {
			containerStatus = containerState.String()
		}
		ctrs = append(ctrs, define.InspectPodContainerInfo{
			ID:    c.ID(),
			Name:  c.Name(),
			State: containerStatus,
		})
		ctrStatuses[c.ID()] = c.state.State
	}
	podState, err := CreatePodStatusResults(ctrStatuses)
	if err != nil {
		return nil, err
	}

	namespaces := map[string]bool{
		"pid":    p.config.UsePodPID,
		"ipc":    p.config.UsePodIPC,
		"net":    p.config.UsePodNet,
		"mount":  p.config.UsePodMount,
		"user":   p.config.UsePodUser,
		"uts":    p.config.UsePodUTS,
		"cgroup": p.config.UsePodCgroupNS,
	}

	sharesNS := []string{}
	for nsStr, include := range namespaces {
		if include {
			sharesNS = append(sharesNS, nsStr)
		}
	}

	inspectData := define.InspectPodData{
		ID:               p.ID(),
		Name:             p.Name(),
		Namespace:        p.Namespace(),
		Created:          p.CreatedTime(),
		State:            podState,
		Hostname:         "",
		Labels:           p.Labels(),
		CreateCgroup:     false,
		CgroupParent:     p.CgroupParent(),
		CgroupPath:       p.state.CgroupPath,
		CreateInfra:      false,
		InfraContainerID: p.state.InfraContainerID,
		InfraConfig:      nil,
		SharedNamespaces: sharesNS,
		NumContainers:    uint(len(containers)),
		Containers:       ctrs,
	}

	return &inspectData, nil
}
