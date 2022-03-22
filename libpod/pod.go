package libpod

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/lock"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
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
// Pod represents a group of containers that may share namespaces
type Pod struct {
	config *PodConfig
	state  *podState

	valid   bool
	runtime *Runtime
	lock    lock.Locker
}

// PodConfig represents a pod's static configuration
type PodConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Namespace the pod is in
	Namespace string `json:"namespace,omitempty"`

	Hostname string `json:"hostname,omitempty"`

	// Labels contains labels applied to the pod
	Labels map[string]string `json:"labels"`
	// CgroupParent contains the pod's Cgroup parent
	CgroupParent string `json:"cgroupParent"`
	// UsePodCgroup indicates whether the pod will create its own Cgroup and
	// join containers to it.
	// If true, all containers joined to the pod will use the pod cgroup as
	// their cgroup parent, and cannot set a different cgroup parent
	UsePodCgroup bool `json:"sharesCgroup,omitempty"`

	// The following UsePod{kernelNamespace} indicate whether the containers
	// in the pod will inherit the namespace from the first container in the pod.
	UsePodPID      bool `json:"sharesPid,omitempty"`
	UsePodIPC      bool `json:"sharesIpc,omitempty"`
	UsePodNet      bool `json:"sharesNet,omitempty"`
	UsePodMount    bool `json:"sharesMnt,omitempty"`
	UsePodUser     bool `json:"sharesUser,omitempty"`
	UsePodUTS      bool `json:"sharesUts,omitempty"`
	UsePodCgroupNS bool `json:"sharesCgroupNS,omitempty"`

	HasInfra bool `json:"hasInfra,omitempty"`

	// Time pod was created
	CreatedTime time.Time `json:"created"`

	// CreateCommand is the full command plus arguments of the process the
	// container has been created with.
	CreateCommand []string `json:"CreateCommand,omitempty"`

	// ID of the pod's lock
	LockID uint32 `json:"lockID"`
}

// podState represents a pod's state
type podState struct {
	// CgroupPath is the path to the pod's Cgroup
	CgroupPath string `json:"cgroupPath"`
	// InfraContainerID is the container that holds pod namespace information
	// Most often an infra container
	InfraContainerID string
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

// ResourceLim returns the cpuset resource limits for the pod
func (p *Pod) ResourceLim() *specs.LinuxResources {
	resCopy := &specs.LinuxResources{}
	empty := &specs.LinuxResources{
		CPU: &specs.LinuxCPU{},
	}
	infra, err := p.runtime.GetContainer(p.state.InfraContainerID)
	if err != nil {
		return empty
	}
	conf := infra.config.Spec
	if err != nil {
		return empty
	}
	if conf.Linux == nil || conf.Linux.Resources == nil {
		return empty
	}
	if err = JSONDeepCopy(conf.Linux.Resources, resCopy); err != nil {
		return nil
	}
	if resCopy.CPU != nil {
		return resCopy
	}

	return empty
}

// CPUPeriod returns the pod CPU period
func (p *Pod) CPUPeriod() uint64 {
	if p.state.InfraContainerID == "" {
		return 0
	}
	infra, err := p.runtime.GetContainer(p.state.InfraContainerID)
	if err != nil {
		return 0
	}
	conf := infra.config.Spec
	if conf != nil && conf.Linux != nil && conf.Linux.Resources != nil && conf.Linux.Resources.CPU != nil && conf.Linux.Resources.CPU.Period != nil {
		return *conf.Linux.Resources.CPU.Period
	}
	return 0
}

// CPUQuota returns the pod CPU quota
func (p *Pod) CPUQuota() int64 {
	if p.state.InfraContainerID == "" {
		return 0
	}
	infra, err := p.runtime.GetContainer(p.state.InfraContainerID)
	if err != nil {
		return 0
	}
	conf := infra.config.Spec
	if conf != nil && conf.Linux != nil && conf.Linux.Resources != nil && conf.Linux.Resources.CPU != nil && conf.Linux.Resources.CPU.Quota != nil {
		return *conf.Linux.Resources.CPU.Quota
	}
	return 0
}

// PidMode returns the PID mode given by the user ex: pod, private...
func (p *Pod) PidMode() string {
	infra, err := p.runtime.GetContainer(p.state.InfraContainerID)
	if err != nil {
		return ""
	}
	ctrSpec := infra.config.Spec
	if ctrSpec != nil && ctrSpec.Linux != nil {
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == specs.PIDNamespace {
				if ns.Path != "" {
					return fmt.Sprintf("ns:%s", ns.Path)
				}
				return "private"
			}
		}
		return "host"
	}
	return ""
}

// PidMode returns the PID mode given by the user ex: pod, private...
func (p *Pod) UserNSMode() string {
	infra, err := p.infraContainer()
	if err != nil {
		return ""
	}
	ctrSpec := infra.config.Spec
	if ctrSpec != nil && ctrSpec.Linux != nil {
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == specs.UserNamespace {
				if ns.Path != "" {
					return fmt.Sprintf("ns:%s", ns.Path)
				}
				return "private"
			}
		}
		return "host"
	}
	return ""
}

// CPUQuota returns the pod CPU quota
func (p *Pod) VolumesFrom() []string {
	if p.state.InfraContainerID == "" {
		return nil
	}
	infra, err := p.runtime.GetContainer(p.state.InfraContainerID)
	if err != nil {
		return nil
	}
	if ctrs, ok := infra.config.Spec.Annotations[define.InspectAnnotationVolumesFrom]; ok {
		return strings.Split(ctrs, ",")
	}
	return nil
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

// CreateCommand returns the os.Args of the process with which the pod has been
// created.
func (p *Pod) CreateCommand() []string {
	return p.config.CreateCommand
}

// CgroupParent returns the pod's Cgroup parent
func (p *Pod) CgroupParent() string {
	return p.config.CgroupParent
}

// SharesPID returns whether containers in pod
// default to use PID namespace of first container in pod
func (p *Pod) SharesPID() bool {
	return p.config.UsePodPID
}

// SharesIPC returns whether containers in pod
// default to use IPC namespace of first container in pod
func (p *Pod) SharesIPC() bool {
	return p.config.UsePodIPC
}

// SharesNet returns whether containers in pod
// default to use network namespace of first container in pod
func (p *Pod) SharesNet() bool {
	return p.config.UsePodNet
}

// SharesMount returns whether containers in pod
// default to use PID namespace of first container in pod
func (p *Pod) SharesMount() bool {
	return p.config.UsePodMount
}

// SharesUser returns whether containers in pod
// default to use user namespace of first container in pod
func (p *Pod) SharesUser() bool {
	return p.config.UsePodUser
}

// SharesUTS returns whether containers in pod
// default to use UTS namespace of first container in pod
func (p *Pod) SharesUTS() bool {
	return p.config.UsePodUTS
}

// SharesCgroup returns whether containers in the pod will default to this pod's
// cgroup instead of the default libpod parent
func (p *Pod) SharesCgroup() bool {
	return p.config.UsePodCgroupNS
}

// Hostname returns the hostname of the pod.
func (p *Pod) Hostname() string {
	return p.config.Hostname
}

// CgroupPath returns the path to the pod's Cgroup
func (p *Pod) CgroupPath() (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if err := p.updatePod(); err != nil {
		return "", err
	}
	if p.state.CgroupPath != "" {
		return p.state.CgroupPath, nil
	}
	if p.state.InfraContainerID == "" {
		return "", errors.Wrap(define.ErrNoSuchCtr, "pod has no infra container")
	}

	id, err := p.infraContainerID()
	if err != nil {
		return "", err
	}

	if id != "" {
		ctr, err := p.infraContainer()
		if err != nil {
			return "", errors.Wrapf(err, "could not get infra")
		}
		if ctr != nil {
			ctr.Start(context.Background(), true)
			cgroupPath, err := ctr.CgroupPath()
			fmt.Println(cgroupPath)
			if err != nil {
				return "", errors.Wrapf(err, "could not get container cgroup")
			}
			p.state.CgroupPath = cgroupPath
			p.save()
			return cgroupPath, nil
		}
	}
	return p.state.CgroupPath, nil
}

// HasContainer checks if a container is present in the pod
func (p *Pod) HasContainer(id string) (bool, error) {
	if !p.valid {
		return false, define.ErrPodRemoved
	}

	return p.runtime.state.PodHasContainer(p, id)
}

// AllContainersByID returns the container IDs of all the containers in the pod
func (p *Pod) AllContainersByID() ([]string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.valid {
		return nil, define.ErrPodRemoved
	}

	return p.runtime.state.PodContainersByID(p)
}

// AllContainers retrieves the containers in the pod
func (p *Pod) AllContainers() ([]*Container, error) {
	if !p.valid {
		return nil, define.ErrPodRemoved
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.allContainers()
}

func (p *Pod) allContainers() ([]*Container, error) {
	return p.runtime.state.PodContainers(p)
}

// HasInfraContainer returns whether the pod will create an infra container
func (p *Pod) HasInfraContainer() bool {
	return p.config.HasInfra
}

// SharesNamespaces checks if the pod has any kernel namespaces set as shared. An infra container will not be
// created if no kernel namespaces are shared.
func (p *Pod) SharesNamespaces() bool {
	return p.SharesPID() || p.SharesIPC() || p.SharesNet() || p.SharesMount() || p.SharesUser() || p.SharesUTS()
}

// infraContainerID returns the infra ID without a lock
func (p *Pod) infraContainerID() (string, error) {
	if err := p.updatePod(); err != nil {
		return "", err
	}
	return p.state.InfraContainerID, nil
}

// InfraContainerID returns the infra container ID for a pod.
// If the container returned is "", the pod has no infra container.
func (p *Pod) InfraContainerID() (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.infraContainerID()
}

// infraContainer is the unlocked version of InfraContainer which returns the infra container
func (p *Pod) infraContainer() (*Container, error) {
	id, err := p.infraContainerID()
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errors.Wrap(define.ErrNoSuchCtr, "pod has no infra container")
	}

	return p.runtime.state.Container(id)
}

// InfraContainer returns the infra container.
func (p *Pod) InfraContainer() (*Container, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.infraContainer()
}

// TODO add pod batching
// Lock pod to avoid lock contention
// Store and lock all containers (no RemoveContainer in batch guarantees cache will not become stale)

// PodContainerStats is an organization struct for pods and their containers
type PodContainerStats struct {
	Pod            *Pod
	ContainerStats map[string]*define.ContainerStats
}

// GetPodStats returns the stats for each of its containers
func (p *Pod) GetPodStats(previousContainerStats map[string]*define.ContainerStats) (map[string]*define.ContainerStats, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if err := p.updatePod(); err != nil {
		return nil, err
	}
	containers, err := p.runtime.state.PodContainers(p)
	if err != nil {
		return nil, err
	}
	newContainerStats := make(map[string]*define.ContainerStats)
	for _, c := range containers {
		newStats, err := c.GetContainerStats(previousContainerStats[c.ID()])
		// If the container wasn't running, don't include it
		// but also suppress the error
		if err != nil && errors.Cause(err) != define.ErrCtrStateInvalid {
			return nil, err
		}
		if err == nil {
			newContainerStats[c.ID()] = newStats
		}
	}
	return newContainerStats, nil
}

// ProcessLabel returns the SELinux label associated with the pod
func (p *Pod) ProcessLabel() (string, error) {
	if !p.HasInfraContainer() {
		return "", nil
	}
	ctr, err := p.infraContainer()
	if err != nil {
		return "", err
	}
	return ctr.ProcessLabel(), nil
}

// initContainers returns the list of initcontainers
// in a pod sorted by create time
func (p *Pod) initContainers() ([]*Container, error) {
	initCons := make([]*Container, 0)
	// the pod is already locked when this is called
	cons, err := p.allContainers()
	if err != nil {
		return nil, err
	}
	// Sort the pod containers by created time
	sort.Slice(cons, func(i, j int) bool { return cons[i].CreatedTime().Before(cons[j].CreatedTime()) })
	// Iterate sorted containers and add ids for any init containers
	for _, c := range cons {
		if len(c.config.InitContainerType) > 0 {
			initCons = append(initCons, c)
		}
	}
	return initCons, nil
}
