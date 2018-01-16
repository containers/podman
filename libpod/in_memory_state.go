package libpod

import (
	"strings"

	"github.com/docker/docker/pkg/truncindex"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/pkg/registrar"
)

// An InMemoryState is a purely in-memory state store
type InMemoryState struct {
	pods         map[string]*Pod
	containers   map[string]*Container
	ctrDepends   map[string][]string
	podNameIndex *registrar.Registrar
	podIDIndex   *truncindex.TruncIndex
	ctrNameIndex *registrar.Registrar
	ctrIDIndex   *truncindex.TruncIndex
}

// NewInMemoryState initializes a new, empty in-memory state
func NewInMemoryState() (State, error) {
	state := new(InMemoryState)

	state.pods = make(map[string]*Pod)
	state.containers = make(map[string]*Container)

	state.podNameIndex = registrar.NewRegistrar()
	state.ctrNameIndex = registrar.NewRegistrar()

	state.podIDIndex = truncindex.NewTruncIndex([]string{})
	state.ctrIDIndex = truncindex.NewTruncIndex([]string{})

	return state, nil
}

// Close the state before shutdown
// This is a no-op as we have no backing disk
func (s *InMemoryState) Close() error {
	return nil
}

// Refresh clears container and pod stats after a reboot
// In-memory state won't survive a reboot so this is a no-op
func (s *InMemoryState) Refresh() error {
	return nil
}

// Container retrieves a container from its full ID
func (s *InMemoryState) Container(id string) (*Container, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	ctr, ok := s.containers[id]
	if !ok {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no container with ID %s found", id)
	}

	return ctr, nil
}

// LookupContainer retrieves a container by full ID, unique partial ID, or name
func (s *InMemoryState) LookupContainer(idOrName string) (*Container, error) {
	if idOrName == "" {
		return nil, ErrEmptyID
	}

	fullID, err := s.ctrNameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			// What was passed is not a name, assume it's an ID
			fullID, err = s.ctrIDIndex.Get(idOrName)
			if err != nil {
				if err == truncindex.ErrNotExist {
					return nil, errors.Wrapf(ErrNoSuchCtr, "no container found with name or ID %s", idOrName)
				}
				return nil, errors.Wrapf(err, "error performing truncindex lookup for ID %s", idOrName)
			}
		} else {
			return nil, errors.Wrapf(err, "error performing registry lookup for ID %s", idOrName)
		}
	}

	ctr, ok := s.containers[fullID]
	if !ok {
		// This should never happen
		return nil, errors.Wrapf(ErrInternal, "mismatch in container ID registry and containers map for ID %s", fullID)
	}

	return ctr, nil
}

// HasContainer checks if a container with the given ID is present in the state
func (s *InMemoryState) HasContainer(id string) (bool, error) {
	if id == "" {
		return false, ErrEmptyID
	}

	_, ok := s.containers[id]

	return ok, nil
}

// AddContainer adds a container to the state
// Containers in a pod cannot be added to the state
func (s *InMemoryState) AddContainer(ctr *Container) error {
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container with ID %s is not valid", ctr.ID())
	}

	_, ok := s.containers[ctr.ID()]
	if ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in state", ctr.ID())
	}

	if ctr.config.Pod != "" {
		return errors.Wrapf(ErrInvalidArg, "cannot add a container that is in a pod with AddContainer, use AddContainerToPod")
	}

	if err := s.ctrNameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
		return errors.Wrapf(err, "error registering container name %s", ctr.Name())
	}

	if err := s.ctrIDIndex.Add(ctr.ID()); err != nil {
		s.ctrNameIndex.Release(ctr.Name())
		return errors.Wrapf(err, "error registering container ID %s", ctr.ID())
	}

	s.containers[ctr.ID()] = ctr

	// Add containers this container depends on
	s.addCtrToDependsMap(ctr.ID(), ctr.config.IPCNsCtr)
	s.addCtrToDependsMap(ctr.ID(), ctr.config.MountNsCtr)
	s.addCtrToDependsMap(ctr.ID(), ctr.config.NetNsCtr)
	s.addCtrToDependsMap(ctr.ID(), ctr.config.PIDNsCtr)
	s.addCtrToDependsMap(ctr.ID(), ctr.config.UserNsCtr)
	s.addCtrToDependsMap(ctr.ID(), ctr.config.UTSNsCtr)
	s.addCtrToDependsMap(ctr.ID(), ctr.config.CgroupNsCtr)

	return nil
}

// RemoveContainer removes a container from the state
// The container will only be removed from the state, not from the pod the container belongs to
func (s *InMemoryState) RemoveContainer(ctr *Container) error {
	// Almost no validity checks are performed, to ensure we can kick
	// misbehaving containers out of the state

	// Ensure we don't remove a container which other containers depend on
	deps, ok := s.ctrDepends[ctr.ID()]
	if ok && len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		return errors.Wrapf(ErrCtrExists, "the following containers depend on container %s: %s", ctr.ID(), depsStr)
	}

	if _, ok := s.containers[ctr.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchCtr, "no container exists in state with ID %s", ctr.ID())
	}

	if err := s.ctrIDIndex.Delete(ctr.ID()); err != nil {
		return errors.Wrapf(err, "error removing container ID from index")
	}
	delete(s.containers, ctr.ID())
	s.ctrNameIndex.Release(ctr.Name())

	delete(s.ctrDepends, ctr.ID())

	s.removeCtrFromDependsMap(ctr.ID(), ctr.config.IPCNsCtr)
	s.removeCtrFromDependsMap(ctr.ID(), ctr.config.MountNsCtr)
	s.removeCtrFromDependsMap(ctr.ID(), ctr.config.NetNsCtr)
	s.removeCtrFromDependsMap(ctr.ID(), ctr.config.PIDNsCtr)
	s.removeCtrFromDependsMap(ctr.ID(), ctr.config.UserNsCtr)
	s.removeCtrFromDependsMap(ctr.ID(), ctr.config.UTSNsCtr)
	s.removeCtrFromDependsMap(ctr.ID(), ctr.config.CgroupNsCtr)

	return nil
}

// UpdateContainer updates a container's state
// As all state is in-memory, no update will be required
// As such this is a no-op
func (s *InMemoryState) UpdateContainer(ctr *Container) error {
	// If the container is invalid, return error
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container with ID %s is not valid", ctr.ID())
	}

	// If the container does not exist, return error
	if _, ok := s.containers[ctr.ID()]; !ok {
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "container with ID %s not found in state", ctr.ID())
	}

	return nil
}

// SaveContainer saves a container's state
// As all state is in-memory, any changes are always reflected as soon as they
// are made
// As such this is a no-op
func (s *InMemoryState) SaveContainer(ctr *Container) error {
	// If the container is invalid, return error
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container with ID %s is not valid", ctr.ID())
	}

	// If the container does not exist, return error
	if _, ok := s.containers[ctr.ID()]; !ok {
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "container with ID %s not found in state", ctr.ID())
	}

	return nil
}

// ContainerInUse checks if the given container is being used by other containers
func (s *InMemoryState) ContainerInUse(ctr *Container) ([]string, error) {
	if !ctr.valid {
		return nil, ErrCtrRemoved
	}

	arr, ok := s.ctrDepends[ctr.ID()]
	if !ok {
		return []string{}, nil
	}

	return arr, nil
}

// AllContainers retrieves all containers from the state
func (s *InMemoryState) AllContainers() ([]*Container, error) {
	ctrs := make([]*Container, 0, len(s.containers))
	for _, ctr := range s.containers {
		ctrs = append(ctrs, ctr)
	}

	return ctrs, nil
}

// Pod retrieves a pod from the state from its full ID
func (s *InMemoryState) Pod(id string) (*Pod, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	pod, ok := s.pods[id]
	if !ok {
		return nil, errors.Wrapf(ErrNoSuchPod, "no pod with id %s found", id)
	}

	return pod, nil
}

// LookupPod retrieves a pod from the state from a full or unique partial ID or
// a full name
func (s *InMemoryState) LookupPod(idOrName string) (*Pod, error) {
	if idOrName == "" {
		return nil, ErrEmptyID
	}

	fullID, err := s.podNameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			// What was passed is not a name, assume it's an ID
			fullID, err = s.podIDIndex.Get(idOrName)
			if err != nil {
				if err == truncindex.ErrNotExist {
					return nil, errors.Wrapf(ErrNoSuchPod, "no pod found with name or ID %s", idOrName)
				}
				return nil, errors.Wrapf(err, "error performing truncindex lookup for ID %s", idOrName)
			}
		} else {
			return nil, errors.Wrapf(err, "error performing registry lookup for ID %s", idOrName)
		}
	}

	pod, ok := s.pods[fullID]
	if !ok {
		// This should never happen
		return nil, errors.Wrapf(ErrInternal, "mismatch in pod ID registry and pod map for ID %s", fullID)
	}

	return pod, nil
}

// HasPod checks if a pod with the given ID is present in the state
func (s *InMemoryState) HasPod(id string) (bool, error) {
	if id == "" {
		return false, ErrEmptyID
	}

	_, ok := s.pods[id]

	return ok, nil
}

// PodContainers retrieves the containers from a pod given the pod's full ID
func (s *InMemoryState) PodContainers(id string) ([]*Container, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	pod, ok := s.pods[id]
	if !ok {
		return nil, errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found", id)
	}

	return pod.GetContainers()
}

// AddPod adds a given pod to the state
// Only empty pods can be added to the state
func (s *InMemoryState) AddPod(pod *Pod) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid and cannot be added", pod.ID())
	}

	if _, ok := s.pods[pod.ID()]; ok {
		return errors.Wrapf(ErrPodExists, "pod with ID %s already exists in state", pod.ID())
	}

	if len(pod.containers) != 0 {
		return errors.Wrapf(ErrInternal, "only empty pods can be added to the state")
	}

	if err := s.podNameIndex.Reserve(pod.Name(), pod.ID()); err != nil {
		return errors.Wrapf(err, "error registering pod name %s", pod.Name())
	}

	if err := s.podIDIndex.Add(pod.ID()); err != nil {
		s.podNameIndex.Release(pod.Name())
		return errors.Wrapf(err, "error registering pod ID %s", pod.ID())
	}

	s.pods[pod.ID()] = pod

	return nil
}

// RemovePod removes a given pod from the state
// Containers within the pod will not be removed or changed
func (s *InMemoryState) RemovePod(pod *Pod) error {
	// Don't make many validity checks to ensure we can kick badly formed
	// pods out of the state

	if _, ok := s.pods[pod.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}

	if err := s.podIDIndex.Delete(pod.ID()); err != nil {
		return errors.Wrapf(err, "error removing pod ID %s from index", pod.ID())
	}
	delete(s.pods, pod.ID())
	s.podNameIndex.Release(pod.Name())

	return nil
}

// UpdatePod updates a pod's state from the backing database
// As in-memory states have no database this is a no-op
func (s *InMemoryState) UpdatePod(pod *Pod) error {
	return nil
}

// AddContainerToPod adds a container to the given pod, also adding it to the
// state
func (s *InMemoryState) AddContainerToPod(pod *Pod, ctr *Container) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid and cannot be added to", pod.ID())
	}
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid and cannot be added to the pod", ctr.ID())
	}

	if ctr.config.Pod != pod.ID() {
		return errors.Wrapf(ErrInvalidArg, "container %s is not in pod %s", ctr.ID(), pod.ID())
	}

	// Add container to pod
	if err := pod.addContainer(ctr); err != nil {
		return err
	}

	// Add container to state
	_, ok := s.containers[ctr.ID()]
	if ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in state", ctr.ID())
	}

	if err := s.ctrNameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
		return errors.Wrapf(err, "error reserving container name %s", ctr.Name())
	}

	if err := s.ctrIDIndex.Add(ctr.ID()); err != nil {
		s.ctrNameIndex.Release(ctr.Name())
		return errors.Wrapf(err, "error releasing container ID %s", ctr.ID())
	}

	s.containers[ctr.ID()] = ctr

	return nil
}

// RemoveContainerFromPod removes the given container from the given pod
// The container is also removed from the state
func (s *InMemoryState) RemoveContainerFromPod(pod *Pod, ctr *Container) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid and containers cannot be removed", pod.ID())
	}
	if !ctr.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid and cannot be removed from the pod", ctr.ID())
	}

	// Is the container in the pod?
	exists, err := pod.HasContainer(ctr.ID())
	if err != nil {
		return errors.Wrapf(err, "error checking for container %s in pod %s", ctr.ID(), pod.ID())
	}
	if !exists {
		return errors.Wrapf(ErrNoSuchCtr, "no container %s in pod %s", ctr.ID(), pod.ID())
	}

	// Remove container from pod
	if err := pod.removeContainer(ctr); err != nil {
		return err
	}

	// Remove container from state
	if _, ok := s.containers[ctr.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchCtr, "no container exists in state with ID %s", ctr.ID())
	}

	if err := s.ctrIDIndex.Delete(ctr.ID()); err != nil {
		return errors.Wrapf(err, "error removing container ID from index")
	}
	delete(s.containers, ctr.ID())
	s.ctrNameIndex.Release(ctr.Name())

	return nil
}

// AllPods retrieves all pods currently in the state
func (s *InMemoryState) AllPods() ([]*Pod, error) {
	pods := make([]*Pod, 0, len(s.pods))
	for _, pod := range s.pods {
		pods = append(pods, pod)
	}

	return pods, nil
}

// Internal Functions

// Add a container to the dependency mappings
func (s *InMemoryState) addCtrToDependsMap(ctrID, dependsID string) {
	if dependsID != "" {
		arr, ok := s.ctrDepends[dependsID]
		if !ok {
			// Do not have a mapping for that container yet
			s.ctrDepends[dependsID] = []string{ctrID}
		} else {
			// Have a mapping for the container
			arr = append(arr, ctrID)
			s.ctrDepends[dependsID] = arr
		}
	}
}

// Remove a container from dependency mappings
func (s *InMemoryState) removeCtrFromDependsMap(ctrID, dependsID string) {
	if dependsID != "" {
		arr, ok := s.ctrDepends[dependsID]
		if !ok {
			// Internal state seems inconsistent
			// But the dependency is definitely gone
			// So just return
			return
		}

		newArr := make([]string, len(arr), 0)

		for _, id := range arr {
			if id != ctrID {
				newArr = append(newArr, id)
			}
		}

		s.ctrDepends[dependsID] = newArr
	}
}
