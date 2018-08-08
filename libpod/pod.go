package libpod

import (
	"time"

	"github.com/containers/storage"
)

// Pod represents a group of containers that are managed together.
// Any operations on a Pod that access state must begin with a call to
// updatePod().
// There is no guarantee that state exists in a readable state before this call,
// and even if it does its contents will be out of date and must be refreshed
// from the database.
// Generally, this requirement applies only to top-level functions; helpers can
// assume their callers handled this requirement. Generally speaking, if a
// function takes the pod lock and accesses any part of state, it should
// updatePod() immediately after locking.
// ffjson: skip
type Pod struct {
	config *PodConfig
	state  *podState

	valid   bool
	runtime *Runtime
	lock    storage.Locker
}

// PodConfig represents a pod's static configuration
type PodConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Namespace the pod is in
	Namespace string `json:"namespace,omitempty"`

	// Labels contains labels applied to the pod
	Labels map[string]string `json:"labels"`
	// CgroupParent contains the pod's CGroup parent
	CgroupParent string `json:"cgroupParent"`
	// UsePodCgroup indicates whether the pod will create its own CGroup and
	// join containers to it.
	// If true, all containers joined to the pod will use the pod cgroup as
	// their cgroup parent, and cannot set a different cgroup parent
	UsePodCgroup bool `json:"usePodCgroup"`

	// Time pod was created
	CreatedTime time.Time `json:"created"`
}

// podState represents a pod's state
type podState struct {
	// CgroupPath is the path to the pod's CGroup
	CgroupPath string `json:"cgroupPath"`
}

// PodInspect represents the data we want to display for
// podman pod inspect
type PodInspect struct {
	Config     *PodConfig
	State      *PodInspectState
	Containers []PodContainerInfo
}

// PodInspectState contains inspect data on the pod's state
type PodInspectState struct {
	CgroupPath string `json:"cgroupPath"`
}

// PodContainerInfo keeps information on a container in a pod
type PodContainerInfo struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// ID retrieves the pod's ID
func (p *Pod) ID() string {
	return p.config.ID
}

// Name retrieves the pod's name
func (p *Pod) Name() string {
	return p.config.Name
}

// Namespace returns the pod's libpod namespace.
// Namespaces are used to logically separate containers and pods in the state.
func (p *Pod) Namespace() string {
	return p.config.Namespace
}

// Labels returns the pod's labels
func (p *Pod) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range p.config.Labels {
		labels[key] = value
	}

	return labels
}

// CreatedTime gets the time when the pod was created
func (p *Pod) CreatedTime() time.Time {
	return p.config.CreatedTime
}

// CgroupParent returns the pod's CGroup parent
func (p *Pod) CgroupParent() string {
	return p.config.CgroupParent
}

// UsePodCgroup returns whether containers in the pod will default to this pod's
// cgroup instead of the default libpod parent
func (p *Pod) UsePodCgroup() bool {
	return p.config.UsePodCgroup
}

// CgroupPath returns the path to the pod's CGroup
func (p *Pod) CgroupPath() (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if err := p.updatePod(); err != nil {
		return "", err
	}

	return p.state.CgroupPath, nil
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
	if !p.valid {
		return nil, ErrPodRemoved
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.allContainers()
}

func (p *Pod) allContainers() ([]*Container, error) {
	return p.runtime.state.PodContainers(p)
}

// TODO add pod batching
// Lock pod to avoid lock contention
// Store and lock all containers (no RemoveContainer in batch guarantees cache will not become stale)

// PodContainerStats is an organization struct for pods and their containers
type PodContainerStats struct {
	Pod            *Pod
	ContainerStats map[string]*ContainerStats
}

// GetPodStats returns the stats for each of its containers
func (p *Pod) GetPodStats(previousContainerStats map[string]*ContainerStats) (map[string]*ContainerStats, error) {
	var (
		ok       bool
		prevStat *ContainerStats
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
	newContainerStats := make(map[string]*ContainerStats)
	for _, c := range containers {
		if prevStat, ok = previousContainerStats[c.ID()]; !ok {
			prevStat = &ContainerStats{}
		}
		newStats, err := c.GetContainerStats(prevStat)
		if err != nil {
			return nil, err
		}
		newContainerStats[c.ID()] = newStats
	}
	return newContainerStats, nil
}
