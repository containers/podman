package libpod

import (
	"strings"

	"github.com/docker/docker/pkg/truncindex"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/pkg/registrar"
)

// TODO: Maybe separate idIndex for pod/containers
// As of right now, partial IDs used in Lookup... need to be unique as well
// This may be undesirable?

// An InMemoryState is a purely in-memory state store
type InMemoryState struct {
	pods          map[string]*Pod
	containers    map[string]*Container
	ctrDepends    map[string][]string
	podContainers map[string]map[string]*Container
	nameIndex     *registrar.Registrar
	idIndex       *truncindex.TruncIndex
}

// NewInMemoryState initializes a new, empty in-memory state
func NewInMemoryState() (State, error) {
	state := new(InMemoryState)

	state.pods = make(map[string]*Pod)
	state.containers = make(map[string]*Container)

	state.ctrDepends = make(map[string][]string)

	state.podContainers = make(map[string]map[string]*Container)

	state.nameIndex = registrar.NewRegistrar()
	state.idIndex = truncindex.NewTruncIndex([]string{})

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

	fullID, err := s.nameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			// What was passed is not a name, assume it's an ID
			fullID, err = s.idIndex.Get(idOrName)
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
		// It's a pod, not a container
		return nil, errors.Wrapf(ErrNoSuchCtr, "name or ID %s is a pod, not a container", idOrName)
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

	// There are potential race conditions with this
	// But in-memory state is intended purely for testing and not production
	// use, so this should be fine.
	depCtrs := ctr.Dependencies()
	for _, depCtr := range depCtrs {
		_, ok = s.containers[depCtr]
		if !ok {
			return errors.Wrapf(ErrNoSuchCtr, "cannot depend on nonexistent container %s", depCtr)
		}
	}

	if err := s.nameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
		return errors.Wrapf(err, "error registering container name %s", ctr.Name())
	}

	if err := s.idIndex.Add(ctr.ID()); err != nil {
		s.nameIndex.Release(ctr.Name())
		return errors.Wrapf(err, "error registering container ID %s", ctr.ID())
	}

	s.containers[ctr.ID()] = ctr

	// Add containers this container depends on
	for _, depCtr := range depCtrs {
		s.addCtrToDependsMap(ctr.ID(), depCtr)
	}

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
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "no container exists in state with ID %s", ctr.ID())
	}

	if err := s.idIndex.Delete(ctr.ID()); err != nil {
		return errors.Wrapf(err, "error removing container ID from index")
	}
	delete(s.containers, ctr.ID())
	s.nameIndex.Release(ctr.Name())

	delete(s.ctrDepends, ctr.ID())

	// Remove us from container dependencies
	depCtrs := ctr.Dependencies()
	for _, depCtr := range depCtrs {
		s.removeCtrFromDependsMap(ctr.ID(), depCtr)
	}

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

	fullID, err := s.nameIndex.Get(idOrName)
	if err != nil {
		if err == registrar.ErrNameNotReserved {
			// What was passed is not a name, assume it's an ID
			fullID, err = s.idIndex.Get(idOrName)
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
		// It's a container not a pod
		return nil, errors.Wrapf(ErrNoSuchPod, "id or name %s is a container not a pod", idOrName)
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

// PodHasContainer checks if the given pod has a container with the given ID
func (s *InMemoryState) PodHasContainer(pod *Pod, ctrID string) (bool, error) {
	if !pod.valid {
		return false, errors.Wrapf(ErrPodRemoved, "pod %s is not valid")
	}

	if ctrID == "" {
		return false, ErrEmptyID
	}

	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return false, errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in state", pod.ID())
	}

	_, ok = podCtrs[ctrID]
	return ok, nil
}

// PodContainersByID returns the IDs of all containers in the given pod
func (s *InMemoryState) PodContainersByID(pod *Pod) ([]string, error) {
	if !pod.valid {
		return nil, errors.Wrapf(ErrPodRemoved, "pod %s is not valid")
	}

	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return nil, errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in state", pod.ID())
	}

	length := len(podCtrs)
	if length == 0 {
		return []string{}, nil
	}

	ctrs := make([]string, 0, length)
	for _, ctr := range podCtrs {
		ctrs = append(ctrs, ctr.ID())
	}

	return ctrs, nil
}

// PodContainers retrieves the containers from a pod
func (s *InMemoryState) PodContainers(pod *Pod) ([]*Container, error) {
	if !pod.valid {
		return nil, errors.Wrapf(ErrPodRemoved, "pod %s is not valid")
	}

	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return nil, errors.Wrapf(ErrNoSuchPod, "no pod with ID %s found in state", pod.ID())
	}

	length := len(podCtrs)
	if length == 0 {
		return []*Container{}, nil
	}

	ctrs := make([]*Container, 0, length)
	for _, ctr := range podCtrs {
		ctrs = append(ctrs, ctr)
	}

	return ctrs, nil
}

// AddPod adds a given pod to the state
func (s *InMemoryState) AddPod(pod *Pod) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid and cannot be added", pod.ID())
	}

	if _, ok := s.pods[pod.ID()]; ok {
		return errors.Wrapf(ErrPodExists, "pod with ID %s already exists in state", pod.ID())
	}

	if _, ok := s.podContainers[pod.ID()]; ok {
		return errors.Wrapf(ErrPodExists, "pod with ID %s already exists in state", pod.ID())
	}

	if err := s.nameIndex.Reserve(pod.Name(), pod.ID()); err != nil {
		return errors.Wrapf(err, "error registering pod name %s", pod.Name())
	}

	if err := s.idIndex.Add(pod.ID()); err != nil {
		s.nameIndex.Release(pod.Name())
		return errors.Wrapf(err, "error registering pod ID %s", pod.ID())
	}

	s.pods[pod.ID()] = pod

	s.podContainers[pod.ID()] = make(map[string]*Container)

	return nil
}

// RemovePod removes a given pod from the state
// Only empty pods can be removed
func (s *InMemoryState) RemovePod(pod *Pod) error {
	// Don't make many validity checks to ensure we can kick badly formed
	// pods out of the state

	if _, ok := s.pods[pod.ID()]; !ok {
		pod.valid = false
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}
	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}
	if len(podCtrs) != 0 {
		return errors.Wrapf(ErrCtrExists, "pod %s is not empty and cannot be removed", pod.ID())
	}

	if err := s.idIndex.Delete(pod.ID()); err != nil {
		return errors.Wrapf(err, "error removing pod ID %s from index", pod.ID())
	}
	delete(s.pods, pod.ID())
	delete(s.podContainers, pod.ID())
	s.nameIndex.Release(pod.Name())

	return nil
}

// RemovePodContainers removes all containers from a pod
// This is used to simultaneously remove a number of containers with
// many interdependencies
// Will only remove containers if no dependencies outside of the pod are present
func (s *InMemoryState) RemovePodContainers(pod *Pod) error {
	if !pod.valid {
		return errors.Wrapf(ErrPodRemoved, "pod %s is not valid", pod.ID())
	}

	// Get pod containers
	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return errors.Wrapf(ErrNoSuchPod, "no pod exists in state with ID %s", pod.ID())
	}

	// Go through container dependencies. Check to see if any are outside the pod.
	for ctr := range podCtrs {
		ctrDeps, ok := s.ctrDepends[ctr]
		if ok {
			for _, dep := range ctrDeps {
				_, ok := podCtrs[dep]
				if !ok {
					return errors.Wrapf(ErrCtrExists, "container %s has dependency %s outside of pod %s", ctr, dep, pod.ID())
				}
			}
		}
	}

	// All dependencies are OK to remove
	// Remove all containers
	s.podContainers[pod.ID()] = make(map[string]*Container)
	for _, ctr := range podCtrs {
		if err := s.idIndex.Delete(ctr.ID()); err != nil {
			return errors.Wrapf(err, "error removing container ID from index")
		}
		s.nameIndex.Release(ctr.Name())

		delete(s.containers, ctr.ID())
		delete(s.ctrDepends, ctr.ID())
	}

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

	// Retrieve pod containers list
	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return errors.Wrapf(ErrPodRemoved, "pod %s not found in state", pod.ID())
	}

	// Is the container already in the pod?
	if _, ok = podCtrs[ctr.ID()]; ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in pod %s", ctr.ID(), pod.ID())
	}

	// There are potential race conditions with this
	// But in-memory state is intended purely for testing and not production
	// use, so this should be fine.
	depCtrs := ctr.Dependencies()
	for _, depCtr := range depCtrs {
		_, ok = s.containers[depCtr]
		if !ok {
			return errors.Wrapf(ErrNoSuchCtr, "cannot depend on nonexistent container %s", depCtr)
		}
	}

	// Add container to state
	_, ok = s.containers[ctr.ID()]
	if ok {
		return errors.Wrapf(ErrCtrExists, "container with ID %s already exists in state", ctr.ID())
	}

	if err := s.nameIndex.Reserve(ctr.Name(), ctr.ID()); err != nil {
		return errors.Wrapf(err, "error reserving container name %s", ctr.Name())
	}

	if err := s.idIndex.Add(ctr.ID()); err != nil {
		s.nameIndex.Release(ctr.Name())
		return errors.Wrapf(err, "error releasing container ID %s", ctr.ID())
	}

	s.containers[ctr.ID()] = ctr

	// Add container to pod containers
	podCtrs[ctr.ID()] = ctr

	// Add containers this container depends on
	for _, depCtr := range depCtrs {
		s.addCtrToDependsMap(ctr.ID(), depCtr)
	}

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

	// Ensure we don't remove a container which other containers depend on
	deps, ok := s.ctrDepends[ctr.ID()]
	if ok && len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		return errors.Wrapf(ErrCtrExists, "the following containers depend on container %s: %s", ctr.ID(), depsStr)
	}

	// Retrieve pod containers
	podCtrs, ok := s.podContainers[pod.ID()]
	if !ok {
		pod.valid = false
		return errors.Wrapf(ErrPodRemoved, "pod %s has been removed", pod.ID())
	}

	// Does the container exist?
	if _, ok := s.containers[ctr.ID()]; !ok {
		ctr.valid = false
		return errors.Wrapf(ErrNoSuchCtr, "container %s does not exist in state", ctr.ID())
	}

	// Is the container in the pod?
	if _, ok := podCtrs[ctr.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchCtr, "container with ID %s not found in pod %s", ctr.ID(), pod.ID())
	}

	// Remove container from state
	if _, ok := s.containers[ctr.ID()]; !ok {
		return errors.Wrapf(ErrNoSuchCtr, "no container exists in state with ID %s", ctr.ID())
	}

	if err := s.idIndex.Delete(ctr.ID()); err != nil {
		return errors.Wrapf(err, "error removing container ID from index")
	}
	delete(s.containers, ctr.ID())
	s.nameIndex.Release(ctr.Name())

	// Remove the container from the pod
	delete(podCtrs, ctr.ID())

	// Remove us from container dependencies
	depCtrs := ctr.Dependencies()
	for _, depCtr := range depCtrs {
		s.removeCtrFromDependsMap(ctr.ID(), depCtr)
	}

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

		newArr := make([]string, 0, len(arr))

		for _, id := range arr {
			if id != ctrID {
				newArr = append(newArr, id)
			}
		}

		s.ctrDepends[dependsID] = newArr
	}
}
