package libpod

import (
	"context"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/parallel"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// startInitContainers starts a pod's init containers.
func (p *Pod) startInitContainers(ctx context.Context) error {
	initCtrs, err := p.initContainers()
	if err != nil {
		return err
	}
	// Now iterate init containers
	for _, initCon := range initCtrs {
		if err := initCon.Start(ctx, true); err != nil {
			return err
		}
		// Check that the init container waited correctly and the exit
		// code is good
		rc, err := initCon.Wait(ctx)
		if err != nil {
			return err
		}
		if rc != 0 {
			return errors.Errorf("init container %s exited with code %d", initCon.ID(), rc)
		}
		// If the container is a once init container, we need to remove it
		// after it runs
		if initCon.config.InitContainerType == define.OneShotInitContainer {
			icLock := initCon.lock
			icLock.Lock()
			var time *uint
			if err := p.runtime.removeContainer(ctx, initCon, false, false, true, time); err != nil {
				icLock.Unlock()
				return errors.Wrapf(err, "failed to remove once init container %s", initCon.ID())
			}
			// Removing a container this way requires an explicit call to clean up the db
			if err := p.runtime.state.RemoveContainerFromPod(p, initCon); err != nil {
				logrus.Errorf("Removing container %s from database: %v", initCon.ID(), err)
			}
			icLock.Unlock()
		}
	}
	return nil
}

// Start starts all containers within a pod.
// It combines the effects of Init() and Start() on a container.
// If a container has already been initialized it will be started,
// otherwise it will be initialized then started.
// Containers that are already running or have been paused are ignored
// All containers are started independently, in order dictated by their
// dependencies.
// An error and a map[string]error are returned.
// If the error is not nil and the map is nil, an error was encountered before
// any containers were started.
// If map is not nil, an error was encountered when starting one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrPodPartialFail.
// If both error and the map are nil, all containers were started successfully.
func (p *Pod) Start(ctx context.Context) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	// Before "regular" containers start in the pod, all init containers
	// must have run and exited successfully.
	if err := p.startInitContainers(ctx); err != nil {
		return nil, err
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
	// If there are no containers without dependencies, we can't start
	// Error out
	if len(graph.noDepNodes) == 0 {
		return nil, errors.Wrapf(define.ErrNoSuchCtr, "no containers in pod %s have no dependencies, cannot start pod", p.ID())
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	// Traverse the graph beginning at nodes with no dependencies
	for _, node := range graph.noDepNodes {
		startNode(ctx, node, false, ctrErrors, ctrsVisited, false)
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrPodPartialFail, "error starting some containers")
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
// Each container will use its own stop timeout.
// Only running containers will be stopped. Paused, stopped, or created
// containers will be ignored.
// If cleanup is true, mounts and network namespaces will be cleaned up after
// the container is stopped.
// All containers are stopped independently. An error stopping one container
// will not prevent other containers being stopped.
// An error and a map[string]error are returned.
// If the error is not nil and the map is nil, an error was encountered before
// any containers were stopped.
// If map is not nil, an error was encountered when stopping one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrPodPartialFail.
// If both error and the map are nil, all containers were stopped without error.
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

	// TODO: There may be cases where it makes sense to order stops based on
	// dependencies. Should we bother with this?

	ctrErrChan := make(map[string]<-chan error)

	// Enqueue a function for each container with the parallel executor.
	for _, ctr := range allCtrs {
		c := ctr
		logrus.Debugf("Adding parallel job to stop container %s", c.ID())
		retChan := parallel.Enqueue(ctx, func() error {
			// TODO: Might be better to batch stop and cleanup
			// together?
			if timeout > -1 {
				if err := c.StopWithTimeout(uint(timeout)); err != nil {
					return err
				}
			} else {
				if err := c.Stop(); err != nil {
					return err
				}
			}

			if cleanup {
				return c.Cleanup(ctx)
			}

			return nil
		})

		ctrErrChan[c.ID()] = retChan
	}

	p.newPodEvent(events.Stop)

	ctrErrors := make(map[string]error)

	// Get returned error for every container we worked on
	for id, channel := range ctrErrChan {
		if err := <-channel; err != nil {
			if errors.Cause(err) == define.ErrCtrStateInvalid || errors.Cause(err) == define.ErrCtrStopped {
				continue
			}
			ctrErrors[id] = err
		}
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrPodPartialFail, "error stopping some containers")
	}
	return nil, nil
}

// Cleanup cleans up all containers within a pod that have stopped.
// All containers are cleaned up independently. An error with one container will
// not prevent other containers being cleaned up.
// An error and a map[string]error are returned.
// If the error is not nil and the map is nil, an error was encountered before
// any containers were cleaned up.
// If map is not nil, an error was encountered when working on one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrPodPartialFail.
// If both error and the map are nil, all containers were paused without error
func (p *Pod) Cleanup(ctx context.Context) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrChan := make(map[string]<-chan error)

	// Enqueue a function for each container with the parallel executor.
	for _, ctr := range allCtrs {
		c := ctr
		logrus.Debugf("Adding parallel job to clean up container %s", c.ID())
		retChan := parallel.Enqueue(ctx, func() error {
			return c.Cleanup(ctx)
		})

		ctrErrChan[c.ID()] = retChan
	}

	ctrErrors := make(map[string]error)

	// Get returned error for every container we worked on
	for id, channel := range ctrErrChan {
		if err := <-channel; err != nil {
			if errors.Cause(err) == define.ErrCtrStateInvalid || errors.Cause(err) == define.ErrCtrStopped {
				continue
			}
			ctrErrors[id] = err
		}
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrPodPartialFail, "error cleaning up some containers")
	}

	return nil, nil
}

// Pause pauses all containers within a pod that are running.
// Only running containers will be paused. Paused, stopped, or created
// containers will be ignored.
// All containers are paused independently. An error pausing one container
// will not prevent other containers being paused.
// An error and a map[string]error are returned.
// If the error is not nil and the map is nil, an error was encountered before
// any containers were paused.
// If map is not nil, an error was encountered when pausing one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrPodPartialFail.
// If both error and the map are nil, all containers were paused without error
func (p *Pod) Pause(ctx context.Context) (map[string]error, error) {
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

	ctrErrChan := make(map[string]<-chan error)

	// Enqueue a function for each container with the parallel executor.
	for _, ctr := range allCtrs {
		c := ctr
		logrus.Debugf("Adding parallel job to pause container %s", c.ID())
		retChan := parallel.Enqueue(ctx, c.Pause)

		ctrErrChan[c.ID()] = retChan
	}

	p.newPodEvent(events.Pause)

	ctrErrors := make(map[string]error)

	// Get returned error for every container we worked on
	for id, channel := range ctrErrChan {
		if err := <-channel; err != nil {
			if errors.Cause(err) == define.ErrCtrStateInvalid || errors.Cause(err) == define.ErrCtrStopped {
				continue
			}
			ctrErrors[id] = err
		}
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrPodPartialFail, "error pausing some containers")
	}
	return nil, nil
}

// Unpause unpauses all containers within a pod that are running.
// Only paused containers will be unpaused. Running, stopped, or created
// containers will be ignored.
// All containers are unpaused independently. An error unpausing one container
// will not prevent other containers being unpaused.
// An error and a map[string]error are returned.
// If the error is not nil and the map is nil, an error was encountered before
// any containers were unpaused.
// If map is not nil, an error was encountered when unpausing one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrPodPartialFail.
// If both error and the map are nil, all containers were unpaused without error.
func (p *Pod) Unpause(ctx context.Context) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrChan := make(map[string]<-chan error)

	// Enqueue a function for each container with the parallel executor.
	for _, ctr := range allCtrs {
		c := ctr
		logrus.Debugf("Adding parallel job to unpause container %s", c.ID())
		retChan := parallel.Enqueue(ctx, c.Unpause)

		ctrErrChan[c.ID()] = retChan
	}

	p.newPodEvent(events.Unpause)

	ctrErrors := make(map[string]error)

	// Get returned error for every container we worked on
	for id, channel := range ctrErrChan {
		if err := <-channel; err != nil {
			if errors.Cause(err) == define.ErrCtrStateInvalid || errors.Cause(err) == define.ErrCtrStopped {
				continue
			}
			ctrErrors[id] = err
		}
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrPodPartialFail, "error unpausing some containers")
	}
	return nil, nil
}

// Restart restarts all containers within a pod that are not paused or in an error state.
// It combines the effects of Stop() and Start() on a container
// Each container will use its own stop timeout.
// All containers are started independently, in order dictated by their
// dependencies. An error restarting one container
// will not prevent other containers being restarted.
// An error and a map[string]error are returned.
// If the error is not nil and the map is nil, an error was encountered before
// any containers were restarted.
// If map is not nil, an error was encountered when restarting one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrPodPartialFail.
// If both error and the map are nil, all containers were restarted without error.
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
		return ctrErrors, errors.Wrapf(define.ErrPodPartialFail, "error stopping some containers")
	}
	p.newPodEvent(events.Stop)
	p.newPodEvent(events.Start)
	return nil, nil
}

// Kill sends a signal to all running containers within a pod.
// Signals will only be sent to running containers. Containers that are not
// running will be ignored. All signals are sent independently, and sending will
// continue even if some containers encounter errors.
// An error and a map[string]error are returned.
// If the error is not nil and the map is nil, an error was encountered before
// any containers were signalled.
// If map is not nil, an error was encountered when signalling one or more
// containers. The container ID is mapped to the error encountered. The error is
// set to ErrPodPartialFail.
// If both error and the map are nil, all containers were signalled successfully.
func (p *Pod) Kill(ctx context.Context, signal uint) (map[string]error, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	allCtrs, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}

	ctrErrChan := make(map[string]<-chan error)

	// Enqueue a function for each container with the parallel executor.
	for _, ctr := range allCtrs {
		c := ctr
		logrus.Debugf("Adding parallel job to kill container %s", c.ID())
		retChan := parallel.Enqueue(ctx, func() error {
			return c.Kill(signal)
		})

		ctrErrChan[c.ID()] = retChan
	}

	p.newPodEvent(events.Kill)

	ctrErrors := make(map[string]error)

	// Get returned error for every container we worked on
	for id, channel := range ctrErrChan {
		if err := <-channel; err != nil {
			if errors.Cause(err) == define.ErrCtrStateInvalid || errors.Cause(err) == define.ErrCtrStopped {
				continue
			}
			ctrErrors[id] = err
		}
	}

	if len(ctrErrors) > 0 {
		return ctrErrors, errors.Wrapf(define.ErrPodPartialFail, "error killing some containers")
	}
	return nil, nil
}

// Status gets the status of all containers in the pod.
// Returns a map of Container ID to Container Status.
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
	noInitCtrs := make([]*Container, 0)
	// Do not add init containers into status
	for _, ctr := range allCtrs {
		if ctrType := ctr.config.InitContainerType; len(ctrType) < 1 {
			noInitCtrs = append(noInitCtrs, ctr)
		}
	}
	return containerStatusFromContainers(noInitCtrs)
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

// Inspect returns a PodInspect struct to describe the pod.
func (p *Pod) Inspect() (*define.InspectPodData, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if err := p.updatePod(); err != nil {
		return nil, err
	}

	containers, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}
	ctrs := make([]define.InspectPodContainerInfo, 0, len(containers))
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
		// Do not add init containers fdr status
		if len(c.config.InitContainerType) < 1 {
			ctrStatuses[c.ID()] = c.state.State
		}
	}
	podState, err := createPodStatusResults(ctrStatuses)
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

	// Infra config contains detailed information on the pod's infra
	// container.
	var infraConfig *define.InspectPodInfraConfig
	var inspectMounts []define.InspectMount
	var devices []define.InspectDevice
	var deviceLimits []define.InspectBlkioThrottleDevice
	var infraSecurity []string
	if p.state.InfraContainerID != "" {
		infra, err := p.runtime.GetContainer(p.state.InfraContainerID)
		if err != nil {
			return nil, err
		}
		infraConfig = new(define.InspectPodInfraConfig)
		infraConfig.HostNetwork = p.NetworkMode() == "host"
		infraConfig.StaticIP = infra.config.ContainerNetworkConfig.StaticIP
		infraConfig.NoManageResolvConf = infra.config.UseImageResolvConf
		infraConfig.NoManageHosts = infra.config.UseImageHosts
		infraConfig.CPUPeriod = p.CPUPeriod()
		infraConfig.CPUQuota = p.CPUQuota()
		infraConfig.CPUSetCPUs = p.ResourceLim().CPU.Cpus
		infraConfig.PidNS = p.NamespaceMode(specs.PIDNamespace)
		infraConfig.UserNS = p.NamespaceMode(specs.UserNamespace)
		namedVolumes, mounts := infra.SortUserVolumes(infra.config.Spec)
		inspectMounts, err = infra.GetMounts(namedVolumes, infra.config.ImageVolumes, mounts)
		infraSecurity = infra.GetSecurityOptions()
		if err != nil {
			return nil, err
		}
		var nodes map[string]string
		devices, err = infra.GetDevices(false, *infra.config.Spec, nodes)
		if err != nil {
			return nil, err
		}
		spec := infra.config.Spec
		if spec.Linux != nil && spec.Linux.Resources != nil && spec.Linux.Resources.BlockIO != nil {
			deviceLimits, err = blkioDeviceThrottle(nodes, spec.Linux.Resources.BlockIO.ThrottleReadBpsDevice)
			if err != nil {
				return nil, err
			}
		}

		if len(infra.config.ContainerNetworkConfig.DNSServer) > 0 {
			infraConfig.DNSServer = make([]string, 0, len(infra.config.ContainerNetworkConfig.DNSServer))
			for _, entry := range infra.config.ContainerNetworkConfig.DNSServer {
				infraConfig.DNSServer = append(infraConfig.DNSServer, entry.String())
			}
		}
		if len(infra.config.ContainerNetworkConfig.DNSSearch) > 0 {
			infraConfig.DNSSearch = make([]string, 0, len(infra.config.ContainerNetworkConfig.DNSSearch))
			infraConfig.DNSSearch = append(infraConfig.DNSSearch, infra.config.ContainerNetworkConfig.DNSSearch...)
		}
		if len(infra.config.ContainerNetworkConfig.DNSOption) > 0 {
			infraConfig.DNSOption = make([]string, 0, len(infra.config.ContainerNetworkConfig.DNSOption))
			infraConfig.DNSOption = append(infraConfig.DNSOption, infra.config.ContainerNetworkConfig.DNSOption...)
		}
		if len(infra.config.HostAdd) > 0 {
			infraConfig.HostAdd = make([]string, 0, len(infra.config.HostAdd))
			infraConfig.HostAdd = append(infraConfig.HostAdd, infra.config.HostAdd...)
		}

		networks, err := infra.networks()
		if err != nil {
			return nil, err
		}
		netNames := make([]string, 0, len(networks))
		for name := range networks {
			netNames = append(netNames, name)
		}
		if len(netNames) > 0 {
			infraConfig.Networks = netNames
		}
		infraConfig.NetworkOptions = infra.config.ContainerNetworkConfig.NetworkOptions
		infraConfig.PortBindings = makeInspectPortBindings(infra.config.ContainerNetworkConfig.PortMappings, nil)
	}

	inspectData := define.InspectPodData{
		ID:                 p.ID(),
		Name:               p.Name(),
		Namespace:          p.Namespace(),
		Created:            p.CreatedTime(),
		CreateCommand:      p.config.CreateCommand,
		State:              podState,
		Hostname:           p.config.Hostname,
		Labels:             p.Labels(),
		CreateCgroup:       p.config.UsePodCgroup,
		CgroupParent:       p.CgroupParent(),
		CgroupPath:         p.state.CgroupPath,
		CreateInfra:        infraConfig != nil,
		InfraContainerID:   p.state.InfraContainerID,
		InfraConfig:        infraConfig,
		SharedNamespaces:   sharesNS,
		NumContainers:      uint(len(containers)),
		Containers:         ctrs,
		CPUSetCPUs:         p.ResourceLim().CPU.Cpus,
		CPUPeriod:          p.CPUPeriod(),
		CPUQuota:           p.CPUQuota(),
		Mounts:             inspectMounts,
		Devices:            devices,
		BlkioDeviceReadBps: deviceLimits,
		VolumesFrom:        p.VolumesFrom(),
		SecurityOpts:       infraSecurity,
	}

	return &inspectData, nil
}
